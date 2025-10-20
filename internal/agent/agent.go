package agent

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-co-op/gocron/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/agent/rustdesk"
	"github.com/open-uem/openuem-agent/internal/commands/deploy"
	"github.com/open-uem/openuem-agent/internal/commands/printers"
	remotedesktop "github.com/open-uem/openuem-agent/internal/commands/remote-desktop"
	"github.com/open-uem/openuem-agent/internal/commands/report"
	"github.com/open-uem/openuem-agent/internal/commands/sftp"
	ansiblecfg "github.com/open-uem/openuem-ansible-config/ansible"
	openuem_utils "github.com/open-uem/utils"
	"github.com/open-uem/wingetcfg/wingetcfg"
	"gopkg.in/ini.v1"
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
	RemoteDesktop          *remotedesktop.RemoteDesktopService
	BadgerDB               *badger.DB
	SFTPServer             *sftp.SFTP
	JetstreamContextCancel context.CancelFunc
	WingetConfigureJob     gocron.Job
}

type JSONActions struct {
	Actions []openuem_nats.DeployAction `json:"actions"`
}

type ProfileConfig struct {
	ProfileID     int                          `yaml:"profileID"`
	Exclusions    []string                     `yaml:"exclusions"`
	Deployments   []string                     `yaml:"deployments"`
	WinGetConfig  *wingetcfg.WinGetCfg         `yaml:"config"`
	AnsibleConfig []ansiblecfg.AnsiblePlaybook `yaml:"ansible"`
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
	r, err := report.RunReport(a.Config.UUID, a.Config.Enabled, a.Config.Debug, a.Config.VNCProxyPort, a.Config.SFTPPort, a.Config.IPAddress, a.Config.SFTPDisabled, a.Config.RemoteAssistanceDisabled, a.Config.TenantID, a.Config.SiteID)
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

	// Get remote config
	if err := a.GetRemoteConfig(); err != nil {
		log.Printf("[ERROR]: could not get remote config %v", err)
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

func (a *Agent) EnableAgentHandler(msg jetstream.Msg) {
	if err := a.ReadConfig(); err != nil {
		log.Printf("[ERROR]: could not read config, reason: %v", err)

		if err := msg.Ack(); err != nil {
			log.Printf("[ERROR]: could not ACK message, reason: %v", err)
		}
		return
	}

	if !a.Config.Enabled {
		// Save property to file
		a.Config.Enabled = true
		if err := a.Config.WriteConfig(); err != nil {
			log.Printf("[ERROR]: could not write agent config: %v", err)

			if err := msg.Ack(); err != nil {
				log.Printf("[ERROR]: could not ACK message, reason: %v", err)
			}
			return
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

	if err := msg.Ack(); err != nil {
		log.Printf("[ERROR]: could not ACK message, reason: %v", err)
	}
}

func (a *Agent) DisableAgentHandler(msg jetstream.Msg) {
	if err := a.ReadConfig(); err != nil {
		log.Printf("[ERROR]: could not read config, reason: %v", err)

		if err := msg.Ack(); err != nil {
			log.Printf("[ERROR]: could not ACK message, reason: %v", err)
		}
		return
	}

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
			log.Printf("[ERROR]: could not write agent config: %v", err)

			if err := msg.Ack(); err != nil {
				log.Printf("[ERROR]: could not ACK message, reason: %v", err)
			}
			return
		}
	}

	if err := msg.Ack(); err != nil {
		log.Printf("[ERROR]: could not ACK message, reason: %v", err)
	}
}

func (a *Agent) RunReportHandler(msg jetstream.Msg) {
	a.ReadConfig()
	r := a.RunReport()
	if r == nil {
		log.Println("[ERROR]: report could not be generated, report has nil value")
		if err := msg.Ack(); err != nil {
			log.Printf("[ERROR]: could not ACK message, reason: %v", err)
		}
		return
	}

	if err := a.SendReport(r); err != nil {
		log.Printf("[ERROR]: report could not be send to NATS server!, reason: %v\n", err)
		if err := msg.Ack(); err != nil {
			log.Printf("[ERROR]: could not ACK message, reason: %v", err)
		}
		return
	}

	if err := msg.Ack(); err != nil {
		log.Printf("[ERROR]: could not ACK message, reason: %v", err)
	}
}

func (a *Agent) StopRemoteDesktopSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.stopvnc."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		if err := msg.Respond([]byte("Remote Desktop service stopped!")); err != nil {
			log.Printf("[ERROR]: could not respond to agent stop remote desktop message, reason: %v\n", err)
		}

		if a.RemoteDesktop != nil {
			a.RemoteDesktop.Stop()
			a.RemoteDesktop = nil
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent stop remote desktop, reason: %v", err)
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
			log.Printf("[ERROR]: could not deploy package using package manager, reason: %v\n", err)
			action.Failed = true
			if err := a.SendDeployResult(&action); err != nil {
				log.Printf("[ERROR]: could not send deploy result to worker, reason: %v\n", err)
				if err := SaveDeploymentNotACK(action); err != nil {
					log.Println("[ERROR]: could not save deployment pending ack to JSON file", err)
				}
			}
			return
		}

		// Send deploy result if succesful
		action.When = time.Now()
		action.Failed = false
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

		if err := deploy.UpdatePackage(action.PackageId); err != nil {
			if strings.Contains(err.Error(), strings.ToLower("0x8A15002B")) {
				log.Println("[INFO]: could not update package using package manager, no updates found", err)
			} else {
				log.Printf("[ERROR]: could not update package using package manager, reason: %v\n", err)
				action.Failed = true
				if err := a.SendDeployResult(&action); err != nil {
					log.Printf("[ERROR]: could not send deploy result to worker, reason: %v\n", err)
				}
			}
			return
		}

		// Send deploy result if succesful
		action.When = time.Now()
		action.Failed = false
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
			log.Printf("[ERROR]: could not uninstall package, reason: %v\n", err)
			action.Failed = false
			if err := a.SendDeployResult(&action); err != nil {
				log.Printf("[ERROR]: could not send deploy result to worker, reason: %v\n", err)
			}
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

func (a *Agent) AgentSettingsSubscribe() error {
	_, err := a.NATSConnection.Subscribe("agent.settings."+a.Config.UUID, func(msg *nats.Msg) {

		data := openuem_nats.AgentSetting{}
		err := json.Unmarshal(msg.Data, &data)
		if err != nil {
			log.Printf("[ERROR]: could not get the agent's settings sent from the console, reason: %v\n", err)
			return
		}

		a.Config.Debug = data.DebugMode
		a.Config.SFTPDisabled = !data.SFTPService
		a.Config.RemoteAssistanceDisabled = !data.RemoteAssistance

		if data.SFTPPort != "" {
			port, err := strconv.Atoi(data.SFTPPort)
			if err != nil {
				log.Println("[ERROR]: the SFTP port is not a valid number")
				return
			}

			if port < 0 || port > 65535 {
				log.Println("[ERROR]: the SFTP port is not a valid port")
				return
			}
		}
		a.Config.SFTPPort = data.SFTPPort

		if data.VNCProxyPort != "" {
			port, err := strconv.Atoi(data.VNCProxyPort)
			if err != nil {
				log.Println("[ERROR]: the VNC proxy port is not a valid number")
				return
			}

			if port < 0 || port > 65535 {
				log.Println("[ERROR]: the VNC proxy port is not a valid port")
				return
			}
		}
		a.Config.VNCProxyPort = data.VNCProxyPort

		if err := a.Config.WriteConfig(); err != nil {
			log.Printf("[ERROR]: could not save the agent's settings, reason: %v\n", err)
			return
		}

		if err := a.Config.SetRestartRequiredFlag(); err != nil {
			log.Printf("[ERROR]: could not set the restart required flag, you may have to restart the agent from the console, reason: %v\n", err)
			return
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent's settings port, reason: %v", err)
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

	// Create JetStream consumer with associated subjects
	go func() {
		a.CreateAgentJetStreamConsumer()
	}()

	// Subscribe to Remote Desktop
	err := a.StartRemoteDesktopSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.StopRemoteDesktopSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	// Subscribe to RustDesk subjects
	err = a.StartRustDeskSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.StopRustDeskSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
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

	err = a.AgentSettingsSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.SetDefaultPrinter()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.RemovePrinter()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	log.Println("[INFO]: Subscribed to NATS subjects!")
}

func (a *Agent) CreateAgentJetStreamConsumer() {
	var ctx context.Context

	js, err := jetstream.New(a.NATSConnection)
	if err != nil {
		log.Printf("[ERROR]: could not intantiate JetStream: %v", err)
		return
	}

	ctx, a.JetstreamContextCancel = context.WithTimeout(context.Background(), SCHEDULETIME_5MIN*time.Minute)
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

}

func (a *Agent) GetRemoteConfig() error {
	if a.NATSConnection == nil {
		return fmt.Errorf("NATS connection is not ready")
	}

	remoteConfigMsg := openuem_nats.RemoteConfigRequest{
		AgentID:  a.Config.UUID,
		TenantID: a.Config.TenantID,
		SiteID:   a.Config.SiteID,
	}

	data, err := json.Marshal(remoteConfigMsg)
	if err != nil {
		return err
	}

	msg, err := a.NATSConnection.Request("agentconfig", data, 10*time.Minute)
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
		a.Config.SFTPDisabled = config.SFTPDisabled
		a.Config.RemoteAssistanceDisabled = config.RemoteAssistanceDisabled
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

func (a *Agent) SetDefaultPrinter() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.defaultprinter."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		printerName := string(msg.Data)
		if printerName == "" {
			log.Println("[ERROR]: printer name cannot be empty")
			return
		}
		log.Printf("[INFO]: set %s printer as default request received", printerName)

		if err := printers.SetDefaultPrinter(printerName); err != nil {
			log.Printf("[ERROR]: could not set printer %s as default, reason: %v\n", printerName, err)
			if err := msg.Respond([]byte(err.Error())); err != nil {
				log.Printf("[ERROR]: could not respond to agent.removeprinter message, reason: %v\n", err)
			}
			return
		}

		if err := msg.Respond(nil); err != nil {
			log.Printf("[ERROR]: could not respond to agent.removeprinter message, reason: %v\n", err)
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to default printer message, reason: %v", err)
	}
	return nil
}

func (a *Agent) RemovePrinter() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.removeprinter."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		printerName := string(msg.Data)
		if printerName == "" {
			log.Println("[ERROR]: printer name cannot be empty")
			return
		}
		log.Printf("[INFO]: remove %s printer request received", printerName)

		if err := printers.RemovePrinter(printerName); err != nil {
			log.Printf("[ERROR]: could not remove %s printer, reason: %v\n", printerName, err)
			if err := msg.Respond(nil); err != nil {
				log.Printf("[ERROR]: could not respond to agent.removeprinter message, reason: %v\n", err)
			}
			return
		}

		if err := msg.Respond(nil); err != nil {
			log.Printf("[ERROR]: could not respond to agent.removeprinter message, reason: %v\n", err)
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to remove printer message, reason: %v", err)
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

func (a *Agent) StartRustDeskSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.rustdesk.start."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {

		rd := rustdesk.New()

		if err := rd.GetInstallationInfo(); err != nil {
			rustdesk.RustDeskRespond(msg, "", err.Error())
			return
		}

		id, err := rd.GetRustDeskID()
		if err != nil {
			rustdesk.RustDeskRespond(msg, "", err.Error())
			return
		}

		if err := rd.SetRustDeskPassword(msg.Data); err != nil {
			rustdesk.RustDeskRespond(msg, "", err.Error())
			return
		}

		if err := rd.Configure(msg.Data); err != nil {
			rustdesk.RustDeskRespond(msg, "", err.Error())
			return
		}

		// if rd.IsFlatpak {
		// 	if err := rd.LaunchRustDesk(); err != nil {
		// 		rustdesk.RustDeskRespond(msg, "", err.Error())
		// 		return
		// 	}
		// }

		// Send ID to the console
		rustdesk.RustDeskRespond(msg, id, "")
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to rustdesk start subject, reason: %v", err)
	}
	return nil
}

func (a *Agent) StopRustDeskSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.rustdesk.stop."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		rd := rustdesk.New()
		if err := rd.GetInstallationInfo(); err != nil {
			rustdesk.RustDeskRespond(msg, "", err.Error())
			return
		}

		if err := rustdesk.KillRustDeskProcess(rd.User.Username); err != nil {
			rustdesk.RustDeskRespond(msg, "", err.Error())
			return
		}

		if err := rustdesk.ConfigRollBack(rd.User.Username, rd.IsFlatpak); err != nil {
			rustdesk.RustDeskRespond(msg, "", err.Error())
			return
		}

		rustdesk.RustDeskRespond(msg, "", "")
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to rustdesk stop subject, reason: %v", err)
	}
	return nil
}
