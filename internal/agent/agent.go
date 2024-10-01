package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/doncicuto/openuem-agent/internal/commands/report"
	"github.com/doncicuto/openuem_nats"
	"github.com/doncicuto/openuem_utils"
	"github.com/go-co-op/gocron/v2"
	"github.com/nats-io/nats.go"
	"golang.org/x/sys/windows"
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
		log.Fatalf("[FATAL]: could not get read agent certificate")
	}
	agent.CertPath = clientCertPath

	clientCertKeyPath := filepath.Join(cwd, "certificates", "agent.key")
	_, err = openuem_utils.ReadPEMPrivateKey(clientCertKeyPath)
	if err != nil {
		log.Fatalf("[FATAL]: could not get read agent private key")
	}
	agent.KeyPath = clientCertKeyPath

	clientCAPath := filepath.Join(cwd, "certificates", "ca.cer")
	caCert, err := openuem_utils.ReadPEMCertificate(clientCAPath)
	if err != nil {
		log.Fatalf("[FATAL]: could not get read CA certificate")
	}
	agent.CACertPath = clientCAPath

	agent.MessageServer = openuem_nats.New(agent.Config.NATSHost, agent.Config.NATSPort, clientCertPath, clientCertKeyPath, caCert)

	return agent
}

func (a *Agent) Start() {
	// Read Agent Config from file
	a.ReadConfig()

	log.Println("[INFO]: agent has been started!")

	a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_5MIN
	a.Config.WriteConfig()

	// Start task scheduler
	a.TaskScheduler.Start()
	log.Println("[INFO]: task scheduler has started!")

	// Try to connect to NATS server and start a reconnect job if failed
	if err := a.MessageServer.Connect(); err != nil {
		a.startNATSConnectJob()
		return
	}

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

	// Run report for the first time after start if agent is enabled
	if a.Config.Enabled {
		r := a.RunReport()

		// Send first report to NATS
		if err := a.SendReport(r); err != nil {
			log.Printf("[ERROR]: report could not be send to NATS server!, reason: %s\n", err.Error())
		}

		// Start scheduled report job every 60 minutes
		a.Config.ExecuteTaskEveryXMinutes = SCHEDULETIME_60MIN
		a.Config.WriteConfig()
		a.startReportJob()
	}
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
	log.Println("[INFO]: agent has been stopped!")
}

func (a *Agent) RunReport() *report.Report {
	start := time.Now()
	log.Println("[INFO]: agent is running a report...")
	r := report.RunReport(a.Config.UUID)
	log.Printf("[INFO]: agent report run took %v\n", time.Since(start))
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
				a.startReportJob()
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

		if err := a.SendReport(r); err != nil {
			log.Printf("[ERROR]: report could not be send to NATS server!, reason: %v\n", err)
			if err := msg.Respond([]byte("Agent Run Report failed!")); err != nil {
				log.Printf("[ERROR]: could not respond to agent force report run, reason: %v\n", err)
			}
		}

		// Test to show message
		if err := runAsUser(`C:\Program Files\OpenUEM Agent\openuem_message.exe`, []string{"info", "--message", "Test", "--title", "Title"}); err != nil {
			log.Printf("[ERROR]: could not show test message to user, reason: %v\n", err)
		}

	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent report subject, reason: %v", err)
	}
	return nil
}

// Reference: https://blog.davidvassallo.me/2022/06/17/golang-in-windows-execute-command-as-another-user/
func getUserToken(pid int) (syscall.Token, error) {
	var err error
	var token syscall.Token

	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		log.Println("Token Process", "err", err)
	}
	defer syscall.CloseHandle(handle)

	// Find process token via win32
	err = syscall.OpenProcessToken(handle, syscall.TOKEN_ALL_ACCESS, &token)

	if err != nil {
		log.Println("Open Token Process", "err", err)
	}
	return token, err
}

const processEntrySize = 568

// Reference: https://stackoverflow.com/questions/36333896/how-to-get-process-id-by-process-name-in-windows-environment
func findProcessByName(name string) (uint32, error) {
	h, e := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if e != nil {
		return 0, e
	}
	p := windows.ProcessEntry32{Size: processEntrySize}
	for {
		e := windows.Process32Next(h, &p)
		if e != nil {
			return 0, e
		}
		if windows.UTF16ToString(p.ExeFile[:]) == name {
			return p.ProcessID, nil
		}
	}
}

// Reference: https://blog.davidvassallo.me/2022/06/17/golang-in-windows-execute-command-as-another-user/
func runAsUser(cmdPath string, args []string) error {
	pid, err := findProcessByName("explorer.exe")
	if err != nil {
		return err
	}

	token, err := getUserToken(int(pid))
	if err != nil {
		return err
	}
	defer token.Close()

	cmd := exec.Command(cmdPath, args...)

	// this is the important bit!
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token:         token,
		CreationFlags: 0x08000000, // Reference: https://stackoverflow.com/questions/42500570/how-to-hide-command-prompt-window-when-using-exec-in-golang
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}
