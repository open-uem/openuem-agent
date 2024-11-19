package agent

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/doncicuto/openuem-agent/internal/commands/deploy"
	"github.com/doncicuto/openuem-agent/internal/commands/report"
	"github.com/doncicuto/openuem-agent/internal/commands/sftp"
	"github.com/doncicuto/openuem-agent/internal/commands/vnc"
	"github.com/doncicuto/openuem_nats"
	"github.com/doncicuto/openuem_utils"
	"github.com/go-co-op/gocron/v2"
	"github.com/nats-io/nats.go"
)

type Agent struct {
	Config         Config
	TaskScheduler  gocron.Scheduler
	ReportJob      gocron.Job
	NATSConnectJob gocron.Job
	MessageServer  *openuem_nats.MessageServer
	CertPath       string
	KeyPath        string
	CACertPath     string
	CACert         *x509.Certificate
	ConsoleCert    *x509.Certificate
	VNCServer      *vnc.VNCServer
	BadgerDB       *badger.DB
	SFTPServer     *sftp.SFTP
}

type JSONActions struct {
	Actions []openuem_nats.DeployAction `json:"actions"`
}

func New() Agent {
	var err error
	agent := Agent{}

	// Task Scheduler
	agent.TaskScheduler, err = gocron.NewScheduler()
	if err != nil {
		log.Fatalf("[FATAL]: could not create the scheduler: %v", err)
	}

	// Read Agent Config from file
	agent.ReadConfig()

	// If it's the initial config, set it and write it
	if agent.Config.UUID == "" {
		agent.SetInitialConfig()
		agent.Config.WriteConfig()
	}

	// Read required certificates and private key
	cwd, err := Getwd()
	if err != nil {
		log.Fatalf("[FATAL]: could not get current working directory")
	}

	clientCertPath := filepath.Join(cwd, "certificates", "agent.cer")
	_, err = openuem_utils.ReadPEMCertificate(clientCertPath)
	if err != nil {
		log.Fatalf("[FATAL]: could not read agent certificate")
	}
	agent.CertPath = clientCertPath

	clientCertKeyPath := filepath.Join(cwd, "certificates", "agent.key")
	_, err = openuem_utils.ReadPEMPrivateKey(clientCertKeyPath)
	if err != nil {
		log.Fatalf("[FATAL]: could not read agent private key")
	}
	agent.KeyPath = clientCertKeyPath

	clientCAPath := filepath.Join(cwd, "certificates", "ca.cer")
	caCert, err := openuem_utils.ReadPEMCertificate(clientCAPath)
	if err != nil {
		log.Fatalf("[FATAL]: could not read CA certificate")
	}
	agent.CACertPath = clientCAPath
	agent.CACert = caCert

	consoleCertPath := filepath.Join(cwd, "certificates", "console.cer")
	agent.ConsoleCert, err = openuem_utils.ReadPEMCertificate(consoleCertPath)
	if err != nil {
		log.Fatalf("[FATAL]: could not read console certificate")
	}

	agent.MessageServer = openuem_nats.New(agent.Config.NATSHost, agent.Config.NATSPort, clientCertPath, clientCertKeyPath, agent.CACert)

	return agent
}

func (a *Agent) Start() {
	// Read Agent Config from file
	// a.ReadConfig()

	log.Println("[INFO]: agent has been started!")

	// Log agent associated user
	currentUser, err := user.Current()
	if err != nil {
		log.Printf("[ERROR]: %v", err)
	}
	log.Printf("[INFO]: agent is run as %s", currentUser.Username)

	a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_5MIN
	a.Config.WriteConfig()

	// Start task scheduler
	a.TaskScheduler.Start()
	log.Println("[INFO]: task scheduler has started!")

	// Start BadgerDB KV
	cwd, err := Getwd()
	a.BadgerDB, err = badger.Open(badger.DefaultOptions(filepath.Join(cwd, "badgerdb")))
	if err != nil {
		log.Printf("[ERROR]: %v", err)
	}

	// Start SFTP server
	a.SFTPServer = sftp.New()
	go func() {
		err := a.SFTPServer.Serve(":2022", a.ConsoleCert, a.CACert, a.BadgerDB)
		if err != nil {
			log.Printf("[ERROR]: %v", err)
		}
	}()
	log.Println("[INFO]: SFTP server has started!")

	// Try to connect to NATS server and start a reconnect job if failed
	if err := a.MessageServer.Connect(); err != nil {
		log.Printf("[ERROR]: %v", err)
		a.startNATSConnectJob()
		return
	}
	a.SubscribeToNATSSubjects()

	// Run report for the first time after start if agent is enabled
	if a.Config.Enabled {
		r := a.RunReport()

		// Send first report to NATS
		if err := a.SendReport(r); err != nil {
			a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_5MIN // Try to send it again in 5 minutes
			log.Printf("[ERROR]: report could not be send to NATS server!, reason: %s\n", err.Error())
		} else {
			// Start scheduled report job every 60 minutes
			a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_60MIN
		}

		a.Config.WriteConfig()
		a.startReportJob()
	}

	a.startPendingACKJob()
}

func (a *Agent) Stop() {
	if a.TaskScheduler != nil {
		if err := a.TaskScheduler.Shutdown(); err != nil {
			log.Printf("[ERROR]: could not close NATS connection, reason: %s\n", err.Error())
		}
	}

	if a.MessageServer != nil {
		if err := a.MessageServer.Close(); err != nil {
			log.Printf("[ERROR]: could not close NATS connection, reason: %s\n", err.Error())
		}
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

	if a.Config.Debug {
		log.Println("========================================================================")
	}

	log.Println("[INFO]: agent is running a report...")
	r := report.RunReport(a.Config.UUID, a.Config.Debug)

	log.Printf("[INFO]: agent report run took %v\n", time.Since(start))

	if a.Config.Debug {
		log.Println("========================================================================")
	}
	return r
}

func (a *Agent) SendReport(r *report.Report) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = a.MessageServer.Connection.Request("report", data, 4*time.Minute)
	if err != nil {
		return err
	}
	return nil
}

func (a *Agent) startReportJob() error {
	var err error
	// Create task for running the agent
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

	// Create task for running the agent
	a.NATSConnectJob, err = a.TaskScheduler.NewJob(
		gocron.DurationJob(
			time.Duration(time.Duration(a.Config.ExecuteTaskEveryXMinutes)*time.Minute),
		),
		gocron.NewTask(
			func() {
				err := a.MessageServer.Connect()
				if err != nil {
					return
				}

				// We have connected
				a.TaskScheduler.RemoveJob(a.NATSConnectJob.ID())
				a.SubscribeToNATSSubjects()
				a.startReportJob()
				a.startPendingACKJob()
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

func (a *Agent) ReportTask() {
	r := a.RunReport()
	if r == nil {
		return
	}
	if err := a.SendReport(r); err != nil {
		a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_5MIN
		a.Config.WriteConfig()
		a.RescheduleReportRunTask()
		log.Printf("[ERROR]: report could not be send to NATS server!, reason: %s\n", err.Error())
		return
	}

	// Report run and sent! Set normal execution time
	a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_60MIN
	a.Config.WriteConfig()
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

func (a *Agent) EnableAgentSubscribe() error {
	_, err := a.MessageServer.Connection.QueueSubscribe("agent.enable."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		a.ReadConfig()

		if !a.Config.Enabled {
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
					a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_60MIN
				}

				a.Config.Enabled = true
				a.Config.WriteConfig()
				a.startReportJob()
			}()

			// Save property to file
			a.Config.Enabled = true
			a.Config.WriteConfig()

			if err := msg.Respond([]byte("Agent Enabled!")); err != nil {
				log.Printf("❌ could not respond to agent enable message, reason: %s\n", err.Error())
			}
		}
	})

	if err != nil {
		return fmt.Errorf("could not subscribe to agent enable subject, reason: %v", err)
	}
	return nil
}

func (a *Agent) DisableAgentSubscribe() error {
	_, err := a.MessageServer.Connection.QueueSubscribe("agent.disable."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		a.ReadConfig()

		if a.Config.Enabled {
			// Stop reporting job
			if err := a.TaskScheduler.RemoveJob(a.ReportJob.ID()); err != nil {
				log.Printf("[INFO]: could not stop report task, reason: %v\n", err)
			} else {
				log.Printf("[INFO]: report task has been removed\n")
			}

			// Save property to file
			a.Config.Enabled = false
			a.Config.WriteConfig()

			if err := msg.Respond([]byte("Agent Disabled!")); err != nil {
				log.Printf("❌ could not respond to agent disable message, reason: %v\n", err)
			}
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent disable subject, reason: %v", err)
	}
	return nil
}

func (a *Agent) RunReportSubscribe() error {
	_, err := a.MessageServer.Connection.QueueSubscribe("agent.report."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		a.ReadConfig()
		r := a.RunReport()
		if r == nil {
			return
		}

		if err := a.SendReport(r); err != nil {
			log.Printf("[ERROR]: report could not be send to NATS server!, reason: %v\n", err)
			if err := msg.Respond([]byte("Agent Run Report failed!")); err != nil {
				log.Printf("[ERROR]: could not respond to agent force report run, reason: %v\n", err)
			}
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent report subject, reason: %v", err)
	}
	return nil
}

func (a *Agent) StartVNCSubscribe() error {
	_, err := a.MessageServer.Connection.QueueSubscribe("agent.startvnc."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {

		loggedOnUser, err := report.GetLoggedOnUsername()
		if err != nil {
			log.Println("[ERROR]: could not get logged on username")
			return
		}

		sid, err := report.GetSID(loggedOnUser)
		if err != nil {
			log.Println("[ERROR]: could not get SID for logged on user")
			return
		}

		// Instantiate new vnc server
		v, err := vnc.New(a.CertPath, a.KeyPath, sid, "1443")
		if err != nil {
			log.Println("[ERROR]: could not get a VNC server")
			return
		}

		// Start VNC server
		a.VNCServer = v
		v.Start()

		if err := msg.Respond([]byte("VNC Started!")); err != nil {
			log.Printf("[ERROR]: could not respond to agent start vnc message, reason: %v\n", err)
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent start vnc, reason: %v", err)
	}
	return nil
}

func (a *Agent) StopVNCSubscribe() error {
	_, err := a.MessageServer.Connection.QueueSubscribe("agent.stopvnc."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		if a.VNCServer != nil {
			a.VNCServer.Stop()
			a.VNCServer = nil
		}

		if err := msg.Respond([]byte("VNC Stopped!")); err != nil {
			log.Printf("[ERROR]: could not respond to agent stop vnc message, reason: %v\n", err)
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent stop vnc, reason: %v", err)
	}
	return nil
}

func (a *Agent) InstallPackageSubscribe() error {
	_, err := a.MessageServer.Connection.Subscribe("agent.installpackage."+a.Config.UUID, func(msg *nats.Msg) {

		action := openuem_nats.DeployAction{}
		err := json.Unmarshal(msg.Data, &action)
		if err != nil {
			log.Printf("[ERROR]: could not get the package id to install, reason: %v\n", err)
			return
		}

		if err := deploy.InstallPackage(action.AgentId, action.PackageId); err != nil {
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
	_, err := a.MessageServer.Connection.Subscribe("agent.updatepackage."+a.Config.UUID, func(msg *nats.Msg) {

		action := openuem_nats.DeployAction{}
		err := json.Unmarshal(msg.Data, &action)
		if err != nil {
			log.Printf("[ERROR]: could not get the package id to update, reason: %v\n", err)
			return
		}

		action.When = time.Now()

		if err := deploy.UpdatePackage(action.AgentId, action.PackageId); err != nil {
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
	_, err := a.MessageServer.Connection.Subscribe("agent.uninstallpackage."+a.Config.UUID, func(msg *nats.Msg) {

		action := openuem_nats.DeployAction{}
		err := json.Unmarshal(msg.Data, &action)
		if err != nil {
			log.Printf("[ERROR]: could not get the package id to uninstall, reason: %v\n", err)
			return
		}

		if err := deploy.UninstallPackage(action.AgentId, action.PackageId); err != nil {
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

func (a *Agent) SendDeployResult(r *openuem_nats.DeployAction) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}

	response, err := a.MessageServer.Connection.Request("deployresult", data, 2*time.Minute)
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
	// Subscribe to NATS subjects
	err := a.EnableAgentSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.DisableAgentSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.RunReportSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.StartVNCSubscribe()
	if err != nil {
		log.Printf("[ERROR]: %v\n", err)
	}

	err = a.StopVNCSubscribe()
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

}
