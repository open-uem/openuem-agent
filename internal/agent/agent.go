package agent

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/deploy"
	"github.com/open-uem/openuem-agent/internal/commands/report"
	"github.com/open-uem/openuem-agent/internal/commands/sftp"
	"github.com/open-uem/openuem-agent/internal/commands/vnc"
	openuem_utils "github.com/open-uem/utils"
	"github.com/open-uem/wingetcfg/wingetcfg"
	"golang.org/x/mod/semver"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v3"
)

type Agent struct {
	Config                 Config
	TaskScheduler          gocron.Scheduler
	ReportJob              gocron.Job
	NATSConnectJob         gocron.Job
	NATSConnection         *nats.Conn
	ServerCertPath         string
	ServerKeyPath          string
	CACert                 *x509.Certificate
	SFTPCert               *x509.Certificate
	VNCServer              *vnc.VNCServer
	BadgerDB               *badger.DB
	SFTPServer             *sftp.SFTP
	JetstreamContextCancel context.CancelFunc
	WingetConfigureJob     gocron.Job
}

type JSONActions struct {
	Actions []openuem_nats.DeployAction `json:"actions"`
}

type ProfileConfig struct {
	ProfileID   int                  `yaml:"profileID"`
	Exclusions  []string             `yaml:"exclusions"`
	Deployments []string             `yaml:"deployments"`
	Config      *wingetcfg.WinGetCfg `yaml:"config"`
}

func New() Agent {
	var err error
	agent := Agent{}

	// Task Scheduler
	agent.TaskScheduler, err = gocron.NewScheduler()
	if err != nil {
		log.Fatalf("[FATAL]: could not create the scheduler: %v", err)
	}

	// Read Agent Config from openuem.ini file
	if err := agent.ReadConfig(); err != nil {
		log.Fatalf("[FATAL]: could not read agent config: %v", err)
	}

	// If it's the initial config, set it and write it
	if agent.Config.UUID == "" {
		agent.SetInitialConfig()
		if err := agent.Config.WriteConfig(); err != nil {
			log.Fatalf("[FATAL]: could not write agent config: %v", err)
		}
	}

	caCert, err := openuem_utils.ReadPEMCertificate(agent.Config.CACert)
	if err != nil {
		log.Fatalf("[FATAL]: could not read CA certificate")
	}
	agent.CACert = caCert

	agent.SFTPCert, err = openuem_utils.ReadPEMCertificate(agent.Config.SFTPCert)
	if err != nil {
		log.Fatalf("[FATAL]: could not read sftp certificate")
	}

	return agent
}

func (a *Agent) Start() {

	log.Println("[INFO]: agent has been started!")

	// Log agent associated user
	currentUser, err := user.Current()
	if err != nil {
		log.Printf("[ERROR]: %v", err)
	}
	log.Printf("[INFO]: agent is run as %s", currentUser.Username)

	a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_5MIN
	if err := a.Config.WriteConfig(); err != nil {
		log.Fatalf("[FATAL]: could not write agent config: %v", err)
	}

	// Agent started so reset restart required flag
	if err := a.Config.ResetRestartRequiredFlag(); err != nil {
		log.Fatalf("[FATAL]: could not reset restart required flag, reason: %v", err)
	}

	// Start task scheduler
	a.TaskScheduler.Start()
	log.Println("[INFO]: task scheduler has started!")

	// Start BadgerDB KV and SFTP server only if port is set
	if a.Config.SFTPPort != "" {
		cwd, err := Getwd()
		if err != nil {
			log.Println("[ERROR]: could not get working directory")
			return
		}

		badgerPath := filepath.Join(cwd, "badgerdb")
		if err := os.RemoveAll(badgerPath); err != nil {
			log.Println("[ERROR]: could not remove badgerdb directory")
			return
		}

		if err := os.MkdirAll(badgerPath, 0660); err != nil {
			log.Println("[ERROR]: could not recreate badgerdb directory")
			return
		}

		a.BadgerDB, err = badger.Open(badger.DefaultOptions(filepath.Join(cwd, "badgerdb")))
		if err != nil {
			log.Printf("[ERROR]: %v", err)
		}

		go func() {
			a.SFTPServer = sftp.New()
			err = a.SFTPServer.Serve(":"+a.Config.SFTPPort, a.SFTPCert, a.CACert, a.BadgerDB)
			if err != nil {
				log.Printf("[ERROR]: %v", err)
			}
			log.Println("[INFO]: SFTP server has started!")
		}()
	}

	// Try to connect to NATS server and start a reconnect job if failed
	a.NATSConnection, err = openuem_nats.ConnectWithNATS(a.Config.NATSServers, a.Config.AgentCert, a.Config.AgentKey, a.Config.CACert)
	if err != nil {
		log.Printf("[ERROR]: %v", err)
		a.startNATSConnectJob()
		return
	}
	a.SubscribeToNATSSubjects()
	log.Println("[INFO]: Subscribed to NATS subjects!")

	// Get remote config
	if err := a.GetRemoteConfig(); err != nil {
		log.Printf("[ERROR]: could not get remote config %v", err)
	}
	log.Println("[INFO]: remote config requested")

	// Run report for the first time after start if agent is enabled
	if a.Config.Enabled {
		r := a.RunReport()
		if r == nil {
			return
		}

		// Send first report to NATS
		if err := a.SendReport(r); err != nil {
			a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_5MIN // Try to send it again in 5 minutes
			log.Printf("[ERROR]: report could not be send to NATS server!, reason: %s\n", err.Error())
		} else {
			// Start scheduled report job with default frequency
			a.Config.ExecuteTaskEveryXMinutes = a.Config.DefaultFrequency
		}

		if err := a.Config.WriteConfig(); err != nil {
			log.Fatalf("[FATAL]: could not write agent config: %v", err)
		}

		a.startReportJob()
	}

	// Start other jobs associated
	a.startPendingACKJob()
	a.startCheckForWinGetProfilesJob()
}

func (a *Agent) Stop() {
	if a.TaskScheduler != nil {
		if err := a.TaskScheduler.Shutdown(); err != nil {
			log.Printf("[ERROR]: could not close NATS connection, reason: %s\n", err.Error())
		}
	}

	if a.NATSConnection != nil {
		a.NATSConnection.Close()
	}

	if a.SFTPServer != nil {
		if err := a.SFTPServer.Server.Close(); err != nil {
			log.Printf("[ERROR]: could not close SFTP server, reason: %s\n", err.Error())
		}
	}

	if a.BadgerDB != nil {
		if err := a.BadgerDB.Close(); err != nil {
			log.Printf("[ERROR]: could not close BadgerDB connection, reason: %s\n", err.Error())
		}
	}
	log.Println("[INFO]: agent has been stopped!")
}

func (a *Agent) RunReport() *report.Report {
	start := time.Now()

	log.Println(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")

	log.Println("[INFO]: agent is running a report...")
	r, err := report.RunReport(a.Config.UUID, a.Config.Enabled, a.Config.Debug, a.Config.VNCProxyPort, a.Config.SFTPPort, a.Config.IPAddress)
	if err != nil {
		return nil
	}

	if r.IP == "" {
		log.Println("[WARN]: agent has no IP address, report won't be sent and we're flagging this so the watchdog can restart the service")

		// Get conf file
		configFile := openuem_utils.GetAgentConfigFile()

		// Open ini file
		cfg, err := ini.Load(configFile)
		if err != nil {
			log.Println("[ERROR]: could not read config file")
			return nil
		}

		cfg.Section("Agent").Key("RestartRequired").SetValue("true")
		if err := cfg.SaveTo(configFile); err != nil {
			log.Println("[ERROR]: could not save RestartRequired flag to config file")
			return nil
		}

		log.Println("[WARN]: the flag to restart the service by the watchdog has been raised")
		return nil
	}

	log.Printf("[INFO]: agent report run took %v\n", time.Since(start))

	log.Println("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")
	return r
}

func (a *Agent) SendReport(r *report.Report) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if a.NATSConnection == nil {
		return fmt.Errorf("NATS connection is not ready")
	}
	_, err = a.NATSConnection.Request("report", data, 4*time.Minute)
	if err != nil {
		return err
	}
	return nil
}

func (a *Agent) startReportJob() error {
	var err error
	// Create task for running the agent
	if a.Config.ExecuteTaskEveryXMinutes == 0 {
		a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_5MIN
	}

	a.ReportJob, err = a.TaskScheduler.NewJob(
		gocron.DurationJob(
			time.Duration(a.Config.ExecuteTaskEveryXMinutes)*time.Minute,
		),
		gocron.NewTask(a.ReportTask),
	)
	if err != nil {
		log.Fatalf("[FATAL]: could not start the agent job: %v", err)
		return err
	}
	log.Printf("[INFO]: new agent job has been scheduled every %d minutes", a.Config.ExecuteTaskEveryXMinutes)
	return nil
}

func (a *Agent) startPendingACKJob() error {
	var err error
	// Create task for running the agent
	_, err = a.TaskScheduler.NewJob(
		gocron.DurationJob(
			SCHEDULETIME_5MIN*time.Minute,
		),
		gocron.NewTask(a.PendingACKTask),
	)
	if err != nil {
		log.Fatalf("[FATAL]: could not start the pending ACK job: %v", err)
		return err
	}
	log.Printf("[INFO]: new pending ACK job has been scheduled every %d minutes", SCHEDULETIME_5MIN)
	return nil
}

func (a *Agent) startNATSConnectJob() error {
	var err error

	if a.Config.ExecuteTaskEveryXMinutes == 0 {
		a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_5MIN
	}

	// Create task for running the agent
	a.NATSConnectJob, err = a.TaskScheduler.NewJob(
		gocron.DurationJob(
			time.Duration(time.Duration(a.Config.ExecuteTaskEveryXMinutes)*time.Minute),
		),
		gocron.NewTask(
			func() {
				a.NATSConnection, err = openuem_nats.ConnectWithNATS(a.Config.NATSServers, a.Config.AgentCert, a.Config.AgentKey, a.Config.CACert)
				if err != nil {
					return
				}

				// We have connected
				a.TaskScheduler.RemoveJob(a.NATSConnectJob.ID())
				a.SubscribeToNATSSubjects()
				a.startReportJob()
				a.startPendingACKJob()
				a.startCheckForWinGetProfilesJob()

				// Get remote config
				if err := a.GetRemoteConfig(); err != nil {
					log.Printf("[ERROR]: could not get remote config %v", err)
				}

			},
		),
	)
	if err != nil {
		log.Fatalf("[FATAL]: could not start the NATS connect job: %v", err)
		return err
	}
	log.Printf("[INFO]: new NATS connect job has been scheduled every %d minutes", a.Config.ExecuteTaskEveryXMinutes)
	return nil
}

func (a *Agent) startCheckForWinGetProfilesJob() error {
	var err error
	// Create task for running the agent

	if a.Config.WingetConfigureFrequency == 0 {
		a.Config.WingetConfigureFrequency = SCHEDULETIME_30MIN
	}

	a.WingetConfigureJob, err = a.TaskScheduler.NewJob(
		gocron.DurationJob(
			time.Duration(a.Config.WingetConfigureFrequency)*time.Minute,
		),
		gocron.NewTask(a.GetWingetConfigureProfiles),
	)
	if err != nil {
		log.Fatalf("[FATAL]: could not start the check for WinGet profiles job, reason: %v", err)
		return err
	}
	log.Printf("[INFO]: new check for WinGet profiles job has been scheduled every %d minutes", a.Config.WingetConfigureFrequency)
	return nil
}

func (a *Agent) ReportTask() {
	r := a.RunReport()
	if r == nil {
		return
	}
	if err := a.SendReport(r); err != nil {
		a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_5MIN
		if err := a.Config.WriteConfig(); err != nil {
			log.Fatalf("[FATAL]: could not write agent config: %v", err)
		}
		a.RescheduleReportRunTask()
		log.Printf("[ERROR]: report could not be send to NATS server!, reason: %s\n", err.Error())
		return
	}

	// Report run and sent! Use default frequency
	a.Config.ExecuteTaskEveryXMinutes = a.Config.DefaultFrequency
	if err := a.Config.WriteConfig(); err != nil {
		log.Fatalf("[FATAL]: could not write agent config: %v", err)
	}
	a.RescheduleReportRunTask()
}

func (a *Agent) PendingACKTask() {
	actions, err := ReadDeploymentNotACK()
	if err != nil {
		log.Printf("[ERROR]: could not read pending deployment ack, reason: %s\n", err.Error())
		return
	}

	j := 0
	for i := 0; i < len(actions); i++ {
		if err := a.SendDeployResult(&actions[i]); err != nil {
			log.Printf("[ERROR]: sending deployment result from task failed!, reason: %s\n", err.Error())
			j = j + 1
		} else {
			actions = slices.Delete(actions, j, j+1)
		}
	}

	if err := SaveDeploymentsNotACK(actions); err != nil {
		log.Printf("[ERROR]: could not save pending deployments ack, reason: %s\n", err.Error())
		return
	}

	if len(actions) > 0 {
		log.Println("[INFO]: updated pending deployment ack in pending_acks.json file")
	}
}

func (a *Agent) RescheduleReportRunTask() {
	a.TaskScheduler.RemoveJob(a.ReportJob.ID())
	a.startReportJob()
}

func (a *Agent) RescheduleWingetConfigureTask() {
	a.TaskScheduler.RemoveJob(a.WingetConfigureJob.ID())
	a.startCheckForWinGetProfilesJob()
}

func (a *Agent) EnableAgentHandler(msg jetstream.Msg) {
	a.ReadConfig()
	msg.Ack()

	if !a.Config.Enabled {
		// Save property to file
		a.Config.Enabled = true
		if err := a.Config.WriteConfig(); err != nil {
			log.Fatalf("[FATAL]: could not write agent config: %v", err)
		}
		log.Println("[INFO]: agent has been enabled!")

		// Run report async
		go func() {
			r := a.RunReport()
			if r == nil {
				return
			}

			// Send report to NATS
			if err := a.SendReport(r); err != nil {
				log.Printf("[ERROR]: report could not be send to NATS server!, reason: %s\n", err.Error())
				a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_5MIN
			} else {
				// Use default frequency
				a.Config.ExecuteTaskEveryXMinutes = a.Config.DefaultFrequency
			}

			// Start report job
			a.startReportJob()
		}()
	}
}

func (a *Agent) DisableAgentHandler(msg jetstream.Msg) {
	a.ReadConfig()
	msg.Ack()

	if a.Config.Enabled {
		log.Println("[INFO]: agent has been disabled!")

		// Stop reporting job
		if err := a.TaskScheduler.RemoveJob(a.ReportJob.ID()); err != nil {
			log.Printf("[INFO]: could not stop report task, reason: %v\n", err)
		} else {
			log.Printf("[INFO]: report task has been removed\n")
		}

		// Save property to file
		a.Config.Enabled = false
		if err := a.Config.WriteConfig(); err != nil {
			log.Fatalf("[FATAL]: could not write agent config: %v", err)
		}
	}
}

func (a *Agent) RunReportHandler(msg jetstream.Msg) {
	a.ReadConfig()
	r := a.RunReport()
	if r == nil {
		log.Println("[ERROR]: report could not be generated, report has nil value")
		msg.Ack()
		return
	}

	if err := a.SendReport(r); err != nil {
		log.Printf("[ERROR]: report could not be send to NATS server!, reason: %v\n", err)
		msg.Ack()
		return
	}
	msg.Ack()
}

func (a *Agent) AgentCertificateHandler(msg jetstream.Msg) {

	data := openuem_nats.AgentCertificateData{}

	if err := json.Unmarshal(msg.Data(), &data); err != nil {
		log.Printf("[ERROR]: could not unmarshal agent certificate data, reason: %v\n", err)
		msg.Ack()
		return
	}

	wd, err := openuem_utils.GetWd()
	if err != nil {
		log.Printf("[ERROR]: could not get working directory, reason: %v\n", err)
		msg.Ack()
	}

	if err := os.MkdirAll(filepath.Join(wd, "certificates"), 0660); err != nil {
		log.Printf("[ERROR]: could not create certificates folder, reason: %v\n", err)
		msg.Ack()
	}

	keyPath := filepath.Join(wd, "certificates", "server.key")

	privateKey, err := x509.ParsePKCS1PrivateKey(data.PrivateKeyBytes)
	if err != nil {
		log.Printf("[ERROR]: could not get private key, reason: %v\n", err)
		msg.Ack()
	}

	err = openuem_utils.SavePrivateKey(privateKey, keyPath)
	if err != nil {
		log.Printf("[ERROR]: could not save agent private key, reason: %v\n", err)
		msg.Ack()
		return
	}
	log.Printf("[INFO]: Agent private key saved in %s", keyPath)

	certPath := filepath.Join(wd, "certificates", "server.cer")
	err = openuem_utils.SaveCertificate(data.CertBytes, certPath)
	if err != nil {
		log.Printf("[ERROR]: could not save agent certificate, reason: %v\n", err)
		msg.Ack()
		return
	}
	log.Printf("[INFO]: Agent certificate saved in %s", keyPath)

	msg.Ack()

	// Finally run a new report to inform that the certificate is ready
	r := a.RunReport()
	if r == nil {
		return
	}
}

func (a *Agent) StopVNCSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.stopvnc."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		if err := msg.Respond([]byte("VNC Stopped!")); err != nil {
			log.Printf("[ERROR]: could not respond to agent stop vnc message, reason: %v\n", err)
		}

		if a.VNCServer != nil {
			a.VNCServer.Stop()
			a.VNCServer = nil
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent stop vnc, reason: %v", err)
	}
	return nil
}

func (a *Agent) RebootSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.reboot."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		log.Println("[INFO]: reboot request received")
		if err := msg.Respond([]byte("Reboot!")); err != nil {
			log.Printf("[ERROR]: could not respond to agent reboot message, reason: %v\n", err)
		}

		action := openuem_nats.RebootOrRestart{}
		if err := json.Unmarshal(msg.Data, &action); err != nil {
			log.Printf("[ERROR]: could not unmarshal to agent reboot message, reason: %v\n", err)
			return
		}

		when := int(time.Until(time.Now()).Seconds())
		if when > 0 {
			if err := exec.Command("cmd", "/C", "shutdown", "/r", "/t", strconv.Itoa(when)).Run(); err != nil {
				fmt.Printf("[ERROR]: could not initiate power off, reason: %v", err)
			}
		} else {
			if err := exec.Command("cmd", "/C", "shutdown", "/r").Run(); err != nil {
				fmt.Printf("[ERROR]: could not initiate power off, reason: %v", err)
			}
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent reboot, reason: %v", err)
	}
	return nil
}

func (a *Agent) PowerOffSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.poweroff."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		log.Println("[INFO]: power off request received")
		if err := msg.Respond([]byte("Power Off!")); err != nil {
			log.Printf("[ERROR]: could not respond to agent power off message, reason: %v\n", err)
			return
		}

		action := openuem_nats.RebootOrRestart{}
		if err := json.Unmarshal(msg.Data, &action); err != nil {
			log.Printf("[ERROR]: could not unmarshal to agent power off message, reason: %v\n", err)
			return
		}

		when := int(time.Until(time.Now()).Seconds())
		if when > 0 {
			if err := exec.Command("cmd", "/C", "shutdown", "/s", "/t", strconv.Itoa(when)).Run(); err != nil {
				fmt.Printf("[ERROR]: could not initiate power off, reason: %v", err)
			}
		} else {
			if err := exec.Command("cmd", "/C", "shutdown", "/s").Run(); err != nil {
				fmt.Printf("[ERROR]: could not initiate shutdown, reason: %v", err)
			}
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent power off, reason: %v", err)
	}
	return nil
}

func (a *Agent) InstallPackageSubscribe() error {
	_, err := a.NATSConnection.Subscribe("agent.installpackage."+a.Config.UUID, func(msg *nats.Msg) {

		action := openuem_nats.DeployAction{}
		err := json.Unmarshal(msg.Data, &action)
		if err != nil {
			log.Printf("[ERROR]: could not get the package id to install, reason: %v\n", err)
			return
		}

		if err := deploy.InstallPackage(action.PackageId); err != nil {
			log.Printf("[ERROR]: could not deploy package using winget, reason: %v\n", err)
			return
		}

		// Send deploy result if succesful
		action.When = time.Now()
		if err := a.SendDeployResult(&action); err != nil {
			log.Printf("[ERROR]: could not send deploy result to worker, reason: %v\n", err)
			if err := SaveDeploymentNotACK(action); err != nil {
				log.Println("[ERROR]: could not save deployment pending ack to JSON file", err)
			}
		}

		// Send a report to update the installed apps
		r := a.RunReport()
		if r == nil {
			return
		}
		if err := a.SendReport(r); err != nil {
			log.Printf("[ERROR]: report could not be send to NATS server!, reason: %s\n", err.Error())
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent install package, reason: %v", err)
	}
	return nil
}

func (a *Agent) UpdatePackageSubscribe() error {
	_, err := a.NATSConnection.Subscribe("agent.updatepackage."+a.Config.UUID, func(msg *nats.Msg) {

		action := openuem_nats.DeployAction{}
		err := json.Unmarshal(msg.Data, &action)
		if err != nil {
			log.Printf("[ERROR]: could not get the package id to update, reason: %v\n", err)
			return
		}

		action.When = time.Now()

		if err := deploy.UpdatePackage(action.PackageId); err != nil {
			if strings.Contains(err.Error(), strings.ToLower("0x8A15002B")) {
				log.Println("[INFO]: could not update package using winget, no updates found", err)
			} else {
				log.Printf("[ERROR]: could not update package using winget, reason: %v\n", err)
			}
			return
		}

		// Send deploy result if succesful
		if err := a.SendDeployResult(&action); err != nil {
			log.Printf("[ERROR]: could not send deploy result to worker, reason: %v\n", err)
			if err := SaveDeploymentNotACK(action); err != nil {
				log.Println("[ERROR]: could not save deployment pending ack to JSON file", err)
			}
		}

		// Send a report to update the installed apps
		r := a.RunReport()
		if r == nil {
			return
		}

		if err := a.SendReport(r); err != nil {
			log.Printf("[ERROR]: report could not be send to NATS server!, reason: %s\n", err.Error())
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent install package, reason: %v", err)
	}
	return nil
}

func (a *Agent) UninstallPackageSubscribe() error {
	_, err := a.NATSConnection.Subscribe("agent.uninstallpackage."+a.Config.UUID, func(msg *nats.Msg) {

		action := openuem_nats.DeployAction{}
		err := json.Unmarshal(msg.Data, &action)
		if err != nil {
			log.Printf("[ERROR]: could not get the package id to uninstall, reason: %v\n", err)
			return
		}

		if err := deploy.UninstallPackage(action.PackageId); err != nil {
			log.Printf("[ERROR]: could not uninstall package using winget, reason: %v\n", err)
			return
		}

		// Send deploy result if succesful
		action.When = time.Now()
		if err := a.SendDeployResult(&action); err != nil {
			log.Printf("[ERROR]: could not send deploy result to worker, reason: %v\n", err)
			if err := SaveDeploymentNotACK(action); err != nil {
				log.Println("[ERROR]: could not save deployment pending ack to JSON file", err)
			}
		}

		// Send a report to update the installed apps
		r := a.RunReport()
		if r == nil {
			return
		}

		if err := a.SendReport(r); err != nil {
			log.Printf("[ERROR]: report could not be send to NATS server!, reason: %s\n", err.Error())
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent uninstall package, reason: %v", err)
	}
	return nil
}

func (a *Agent) NewConfigSubscribe() error {
	_, err := a.NATSConnection.Subscribe("agent.newconfig", func(msg *nats.Msg) {

		config := openuem_nats.Config{}
		err := json.Unmarshal(msg.Data, &config)
		if err != nil {
			log.Printf("[ERROR]: could not get new config to apply, reason: %v\n", err)
			return
		}

		a.Config.DefaultFrequency = config.AgentFrequency

		// Should we re-schedule agent report?
		if a.Config.ExecuteTaskEveryXMinutes != SCHEDULETIME_5MIN {
			a.Config.ExecuteTaskEveryXMinutes = a.Config.DefaultFrequency
			a.RescheduleReportRunTask()
		}

		// Should we re-schedule winget configure task?
		if config.WinGetFrequency != 0 {
			a.Config.WingetConfigureFrequency = config.WinGetFrequency
			a.RescheduleWingetConfigureTask()
		}

		if err := a.Config.WriteConfig(); err != nil {
			log.Fatalf("[FATAL]: could not write agent config: %v", err)
		}
		log.Println("[INFO]: new config has been set from console")
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent uninstall package, reason: %v", err)
	}
	return nil
}

func (a *Agent) EnableDebugModeSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.enabledebug."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		log.Println("[INFO]: enable debug received")

		a.Config.Debug = true

		if err := a.Config.WriteConfig(); err != nil {
			log.Fatalf("[FATAL]: could not write agent config: %v", err)
		}

		log.Println("[INFO]: agent debug mode has been enabled!")

		if err := msg.Respond([]byte("enabled")); err != nil {
			log.Printf("[ERROR]: could not respond to agent enable debug mode message, reason: %v\n", err)
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to enable debug mode message, reason: %v", err)
	}
	return nil
}

func (a *Agent) DisableDebugModeSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.disabledebug."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		log.Println("[INFO]: disable debug received")

		a.Config.Debug = false

		if err := a.Config.WriteConfig(); err != nil {
			log.Fatalf("[FATAL]: could not write agent config: %v", err)
		}

		log.Println("[INFO]: agent debug mode has been disabled!")

		if err := msg.Respond([]byte("disabled")); err != nil {
			log.Printf("[ERROR]: could not respond to agent disable debug mode message, reason: %v\n", err)
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to disable debug mode message, reason: %v", err)
	}
	return nil
}

func (a *Agent) SendDeployResult(r *openuem_nats.DeployAction) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}

	response, err := a.NATSConnection.Request("deployresult", data, 2*time.Minute)
	if err != nil {
		return err
	}

	responseData := string(response.Data)
	if len(responseData) > 0 {
		return fmt.Errorf("%s", responseData)
	}

	return nil
}

func ReadDeploymentNotACK() ([]openuem_nats.DeployAction, error) {
	cwd, err := Getwd()
	if err != nil {
		return nil, err
	}

	filename := filepath.Join(cwd, "pending_acks.json")
	jsonFile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	jActions := JSONActions{}
	if len(byteValue) > 0 {
		err = json.Unmarshal(byteValue, &jActions)
		if err != nil {
			return nil, err
		}
		return jActions.Actions, nil
	}

	return []openuem_nats.DeployAction{}, nil
}

func SaveDeploymentsNotACK(actions []openuem_nats.DeployAction) error {
	cwd, err := Getwd()
	if err != nil {
		return err
	}

	filename := filepath.Join(cwd, "pending_acks.json")
	jsonFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	jActions := JSONActions{}
	jActions.Actions = actions

	byteValue, err := json.MarshalIndent(jActions, "", " ")
	if err != nil {
		return err
	}

	_, err = jsonFile.Write(byteValue)
	if err != nil {
		return err
	}

	return nil
}

func SaveDeploymentNotACK(action openuem_nats.DeployAction) error {
	var actions []openuem_nats.DeployAction
	cwd, err := Getwd()
	if err != nil {
		return err
	}

	filename := filepath.Join(cwd, "pending_acks.json")
	jsonFile, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return err
	}

	jActions := JSONActions{}

	if len(byteValue) > 0 {
		err = json.Unmarshal(byteValue, &jActions)
		if err != nil {
			return err
		}
		actions = jActions.Actions
	}

	actions = append(actions, action)

	if err := SaveDeploymentsNotACK(actions); err != nil {
		return err
	}

	return nil
}

func (a *Agent) SubscribeToNATSSubjects() {
	var ctx context.Context

	js, err := jetstream.New(a.NATSConnection)
	if err != nil {
		log.Printf("[ERROR]: could not intantiate JetStream: %v", err)
		return
	}

	ctx, a.JetstreamContextCancel = context.WithTimeout(context.Background(), 60*time.Minute)
	s, err := js.Stream(ctx, "AGENTS_STREAM")

	if err != nil {
		log.Printf("[ERROR]: could not get stream AGENTS_STREAM: %v\n", err)
		return
	}

	consumerConfig := jetstream.ConsumerConfig{
		Durable: "AgentConsumer" + a.Config.UUID,
		FilterSubjects: []string{
			"agent.certificate." + a.Config.UUID, "agent.enable." + a.Config.UUID,
			"agent.disable." + a.Config.UUID, "agent.report." + a.Config.UUID,
			"agent.update.updater." + a.Config.UUID, "agent.rollback.updater." + a.Config.UUID,
		},
	}

	if len(strings.Split(a.Config.NATSServers, ",")) > 1 {
		consumerConfig.Replicas = int(math.Min(float64(len(strings.Split(a.Config.NATSServers, ","))), 5))
	}

	c1, err := s.CreateOrUpdateConsumer(ctx, consumerConfig)
	if err != nil {
		log.Printf("[ERROR]: could not create Jetstream consumer: %v", err)
		return
	}

	// TODO stop consume context ()
	_, err = c1.Consume(a.JetStreamAgentHandler, jetstream.ConsumeErrHandler(func(consumeCtx jetstream.ConsumeContext, err error) {
		log.Printf("[ERROR]: consumer error: %v", err)
	}))
	if err != nil {
		log.Printf("[ERROR]: could not start Agent consumer: %v", err)
		return
	}
	log.Println("[INFO]: Agent consumer is ready to serve")

	// Subscribe to VNC only if port is set
	if a.Config.VNCProxyPort != "" {
		err = a.StartVNCSubscribe()
		if err != nil {
			log.Printf("[ERROR]: %v\n", err)
		}

		err = a.StopVNCSubscribe()
		if err != nil {
			log.Printf("[ERROR]: %v\n", err)
		}
	}

	err = a.InstallPackageSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.UpdatePackageSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.UninstallPackageSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.NewConfigSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.PowerOffSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.RebootSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.EnableDebugModeSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.DisableDebugModeSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}
}

func (a *Agent) GetRemoteConfig() error {
	if a.NATSConnection == nil {
		return fmt.Errorf("NATS connection is not ready")
	}

	msg, err := a.NATSConnection.Request("agentconfig", nil, 10*time.Minute)
	if err != nil {
		return err
	}

	if msg == nil || msg.Data == nil {
		return fmt.Errorf("no config was received")
	}

	config := openuem_nats.Config{}

	if err := json.Unmarshal(msg.Data, &config); err != nil {
		return err
	}

	if config.Ok {
		a.Config.DefaultFrequency = config.AgentFrequency
		a.Config.WingetConfigureFrequency = config.WinGetFrequency
		if err := a.Config.WriteConfig(); err != nil {
			log.Fatalf("[FATAL]: could not write agent config: %v", err)
		}

		if a.Config.Debug {
			log.Printf("[DEBUG]: new default frequency is %d", a.Config.DefaultFrequency)
		}
	}
	return nil
}

func (a *Agent) JetStreamAgentHandler(msg jetstream.Msg) {
	if msg.Subject() == "agent.enable."+a.Config.UUID {
		a.EnableAgentHandler(msg)
	}

	if msg.Subject() == "agent.disable."+a.Config.UUID {
		a.DisableAgentHandler(msg)
	}

	if msg.Subject() == "agent.report."+a.Config.UUID {
		a.RunReportHandler(msg)
	}

	if msg.Subject() == "agent.certificate."+a.Config.UUID {
		a.AgentCertificateHandler(msg)
	}
}

func (a *Agent) GetServerCertificate() {

	cwd, err := openuem_utils.GetWd()
	if err != nil {
		log.Println("[ERROR]: could not get current working directory")
	}
	serverCertPath := filepath.Join(cwd, "certificates", "server.cer")
	_, err = openuem_utils.ReadPEMCertificate(serverCertPath)
	if err != nil {
		log.Printf("[ERROR]: could not read server certificate")
	} else {
		a.ServerCertPath = serverCertPath
	}

	serverKeyPath := filepath.Join(cwd, "certificates", "server.key")
	_, err = openuem_utils.ReadPEMPrivateKey(serverKeyPath)
	if err != nil {
		log.Printf("[ERROR]: could not read server private key")
	} else {
		a.ServerKeyPath = serverKeyPath
	}
}

func (a *Agent) GetWingetConfigureProfiles() {
	if a.Config.Debug {
		log.Println("[DEBUG]: running task WinGet profiles job")
	}

	profiles := []ProfileConfig{}

	profileRequest := openuem_nats.WingetCfgProfiles{
		AgentID: a.Config.UUID,
	}

	if a.Config.Debug {
		log.Println("[DEBUG]: going to send a wingetcfg.profile request")
	}

	data, err := json.Marshal(profileRequest)
	if err != nil {
		log.Printf("[ERROR]: could not marshal profile request, reason: %v", err)
	}

	if a.Config.Debug {
		log.Println("[DEBUG]: wingetcfg.profile sending request")
	}

	msg, err := a.NATSConnection.Request("wingetcfg.profiles", data, 5*time.Minute)
	if err != nil {
		log.Printf("[ERROR]: could not send request to agent worker, reason: %v", err)
	}

	if a.Config.Debug {
		log.Println("[DEBUG]: wingetcfg.profile request sent")
		if msg.Data != nil {
			log.Println("[DEBUG]: received wingetcfg.profile response")
		}
	}

	if err := yaml.Unmarshal(msg.Data, &profiles); err != nil {
		log.Printf("[ERROR]: could not unmarshal profiles response from agent worker, reason: %v", err)
	}

	if a.Config.Debug {
		log.Println("[DEBUG]: wingetcfg.profile response unmarshalled")
	}

	for _, p := range profiles {
		if a.Config.Debug {
			log.Println("[DEBUG]: wingetcfg.profile to be unmarshalled")
		}

		cfg, err := yaml.Marshal(p.Config)
		if err != nil {
			log.Printf("[ERROR]: could not marshal YAML file with winget configuration, reason: %v", err)
			continue
		}

		if a.Config.Debug {
			log.Println("[DEBUG]: we're going to apply the configuration")
		}

		if err := a.ApplyConfiguration(p.ProfileID, cfg, p.Exclusions, p.Deployments); err != nil {
			// TODO inform that this profile has an error to agent worker
			log.Printf("[ERROR]: could not apply YAML configuration file with winget, reason: %v", err)
			continue
		}
	}
}

func (a *Agent) ApplyConfiguration(profileID int, config []byte, exclusions, deployments []string) error {
	var cfg wingetcfg.WinGetCfg

	if err := yaml.Unmarshal(config, &cfg); err != nil {
		return err
	}

	cwd, err := openuem_utils.GetWd()
	if err != nil {
		log.Printf("[ERROR]: could not get working directory, reason %v", err)
		return err
	}

	powershellPath := "C:\\Program Files\\PowerShell\\7\\pwsh.exe"

	// If PowerShell 7 is not installed install it with winget
	if _, err := os.Stat(powershellPath); errors.Is(err, os.ErrNotExist) {
		if err := deploy.InstallPackage("Microsoft.PowerShell"); err != nil {
			log.Printf("[ERROR]: WinGet configuration requires PowerShell 7 and it could not be installed, reason %v", err)
			return err
		}

		// Notify, OpenUEM that a new package has been deployed due to winget configure
		if err := a.SendWinGetCfgDeploymentReport("Microsoft.PowerShell", "PowerShell 7-x64", "install"); err != nil {
			return err
		}
	}

	if a.Config.Debug {
		log.Println("[DEBUG]: PowerShell 7 is installed")
	}

	// Check PowerShell 7 version
	// Ref: https://stackoverflow.com/questions/1825585/determine-installed-powershell-version
	out, err := exec.Command(powershellPath, "-Command", "Get-ItemPropertyValue", "-Path", "HKLM:\\SOFTWARE\\Microsoft\\PowerShellCore\\InstalledVersions\\*", "-Name", "SemanticVersion").Output()
	if err != nil {
		log.Printf("[ERROR]: could not get PowerShell 7 version with %s %s %s %s %s %s %s, reason %v", powershellPath, "-Command", "Get-ItemPropertyValue", "-Path", "HKLM:\\SOFTWARE\\Microsoft\\PowerShellCore\\InstalledVersions\\*", "-Name", "SemanticVersion", err)
		return err
	}

	if a.Config.Debug {
		log.Println("[DEBUG]: got PowerShell 7 version")
	}

	// if PowerShell version 7 is lower than 7.4.6 upgrade it
	if semver.Compare("v"+strings.TrimSpace(string(out)), "v7.4.6") < 0 {
		if _, err := os.Stat(powershellPath); errors.Is(err, os.ErrNotExist) {
			if err := deploy.UpdatePackage("Microsoft.PowerShell"); err != nil {
				log.Printf("[ERROR]: WinGet configuration requires PowerShell 7 and it could not be installed, reason %v", err)
				return err
			}

			// Notify, OpenUEM that a new package has been deployed due to winget configure
			if err := a.SendWinGetCfgDeploymentReport("Microsoft.PowerShell", "PowerShell 7-x64", "install"); err != nil {
				return err
			}
		}
	}

	if a.Config.Debug {
		log.Println("[DEBUG]: PowerShell 7 version was compared")
	}

	// Check if packages were explicitely deleted and profile tries to install it again
	explicitelyDeleted := deploy.GetExplicitelyDeletedPackages(deployments)

	if a.Config.Debug {
		log.Println("[DEBUG]: explicitely deleted packages", explicitelyDeleted)
	}

	if err := deploy.RemovePackagesFromCfg(&cfg, explicitelyDeleted); err != nil {
		log.Printf("[ERROR]: could not remove explicitely deleted from config file, reason: %v", err)
	}

	if a.Config.Debug {
		log.Printf("[DEBUG]: config after removing explicitely deleted: +%v", cfg.Properties.Resources)
	}

	// Notify which packages has been explicitely deleted to remove it from console
	a.SendWinGetCfgExcludedPackage(explicitelyDeleted)

	if a.Config.Debug {
		log.Println("[DEBUG]: exclusions received from worker", exclusions)
	}

	// Remove exclusions to avoid reinstalling of explicitely deleted packages
	if err := deploy.RemovePackagesFromCfg(&cfg, exclusions); err != nil {
		log.Printf("[ERROR]: could not remove exclusions from config file, reason: %v", err)
	}

	if a.Config.Debug {
		log.Printf("[DEBUG]: config after removing exclusions: +%v", cfg)
	}

	// Run configuration
	scriptPath := filepath.Join(cwd, "powershell", "configure.ps1")
	configPath := filepath.Join(cwd, "powershell", fmt.Sprintf("openuem.%s.winget", uuid.New()))
	if err := cfg.WriteConfigFile(configPath); err != nil {
		return err
	}

	// Remove powershell configuration file if debug is not enabled
	defer func() {
		if !a.Config.Debug {
			if err := os.Remove(configPath); err != nil {
				log.Printf("[ERROR]: could not remove %s", configPath)
			}
		}
	}()

	if a.Config.Debug {
		log.Println("[DEBUG]: Configure file was created")
	}

	log.Println("[INFO]: received a request to apply a configuration profile")

	cmd := exec.Command(powershellPath, scriptPath, configPath)

	executeErr := cmd.Run()
	errData := ""
	if executeErr != nil {
		log.Println("[ERROR]: configuration profile could not be applied")
		data, err := os.ReadFile("C:\\Program Files\\OpenUEM Agent\\logs\\wingetcfg.txt")
		if err != nil {
			log.Println("[ERROR]: could not read wingetcfg.txt log")
		}
		errData = string(data)
	} else {
		log.Println("[INFO]: winget configuration have finished successfully")
	}

	// Report if application was successful or not
	if err := a.SendWinGetCfgProfileApplicationReport(profileID, a.Config.UUID, executeErr == nil, errData); err != nil {
		log.Println("[ERROR]: could not report if application was applied succesfully or no")
	}

	// Check if packages have been installed (or uninstalled) and notify the agent worker
	a.CheckIfCfgPackagesInstalled(cfg)

	return nil
}

func (a *Agent) CheckIfCfgPackagesInstalled(cfg wingetcfg.WinGetCfg) {
	for _, r := range cfg.Properties.Resources {
		if r.Resource == wingetcfg.WinGetPackageResource {
			packageID := r.Settings["id"].(string)
			packageName := r.Directives.Description
			if r.Settings["Ensure"].(string) == "Present" {
				if deploy.IsWinGetPackageInstalled(packageID) {
					if err := a.SendWinGetCfgDeploymentReport(packageID, packageName, "install"); err != nil {
						log.Printf("[ERROR]: could not send WinGetCfg deployment report, reason: %v", err)
						continue
					}
				}
			} else {
				if !deploy.IsWinGetPackageInstalled(packageID) {
					if err := a.SendWinGetCfgDeploymentReport(packageID, packageName, "uninstall"); err != nil {
						log.Printf("[ERROR]: could not send WinGetCfg deployment report, reason: %v", err)
						continue
					}
				}
			}
		}
	}
}

func (a *Agent) SendWinGetCfgDeploymentReport(packageID, packageName, action string) error {
	// Notify, OpenUEM that a new package has been deployed
	deployment := openuem_nats.DeployAction{
		AgentId:     a.Config.UUID,
		PackageId:   packageID,
		PackageName: packageName,
		When:        time.Now(),
		Action:      action,
	}

	data, err := json.Marshal(deployment)
	if err != nil {
		return err
	}

	if _, err := a.NATSConnection.Request("wingetcfg.deploy", data, 2*time.Minute); err != nil {
		return err
	}

	return nil
}

func (a *Agent) SendWinGetCfgProfileApplicationReport(profileID int, agentID string, success bool, errData string) error {
	// Notify worker if application was succesful or not
	deployment := openuem_nats.WingetCfgReport{
		ProfileID: profileID,
		AgentID:   agentID,
		Success:   success,
		Error:     errData,
	}

	data, err := json.Marshal(deployment)
	if err != nil {
		return err
	}

	if _, err := a.NATSConnection.Request("wingetcfg.report", data, 2*time.Minute); err != nil {
		return err
	}

	return nil
}

func (a *Agent) SendWinGetCfgExcludedPackage(packageIDs []string) {
	for _, id := range packageIDs {
		deployment := openuem_nats.DeployAction{
			AgentId:   a.Config.UUID,
			PackageId: id,
		}

		data, err := json.Marshal(deployment)
		if err != nil {
			log.Printf("[ERROR]: could not marshal package exclude for package %s and agent %s", id, a.Config.UUID)
			return
		}

		if _, err := a.NATSConnection.Request("wingetcfg.exclude", data, 2*time.Minute); err != nil {
			log.Printf("[ERROR]: could not send package exclude for package %s and agent %s", id, a.Config.UUID)
		}
	}
}
