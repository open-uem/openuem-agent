//go:build linux

package agent

import (
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-co-op/gocron/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/sftp"
	"github.com/open-uem/openuem-agent/internal/commands/vnc"
	openuem_utils "github.com/open-uem/utils"
)

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

func (a *Agent) StartVNCSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.startvnc."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {

		// Instantiate new vnc server, but first try to check if certificates are there
		a.GetServerCertificate()
		if a.ServerCertPath == "" || a.ServerKeyPath == "" {
			log.Println("[ERROR]: VNC requires a server certificate that it's not ready")
			return
		}

		v, err := vnc.New(a.ServerCertPath, a.ServerKeyPath, "", a.Config.VNCProxyPort)
		if err != nil {
			log.Println("[ERROR]: could not get a VNC server")
			return
		}

		// Unmarshal data
		var vncConn openuem_nats.VNCConnection
		if err := json.Unmarshal(msg.Data, &vncConn); err != nil {
			log.Println("[ERROR]: could not unmarshall VNC connection")
			return
		}

		// Start VNC server
		a.VNCServer = v
		v.Start(vncConn.PIN, vncConn.NotifyUser)

		if err := msg.Respond([]byte("VNC Started!")); err != nil {
			log.Printf("[ERROR]: could not respond to agent start vnc message, reason: %v\n", err)
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent start vnc, reason: %v", err)
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

		when := int(time.Until(action.Date).Minutes())
		if when > 0 {
			if err := exec.Command("shutdown", "-r", strconv.Itoa(when)).Run(); err != nil {
				fmt.Printf("[ERROR]: could not initiate power off, reason: %v", err)
			}
		} else {
			if err := exec.Command("shutdown", "-r", "now").Run(); err != nil {
				fmt.Printf("[ERROR]: could not initiate shutdown, reason: %v", err)
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

		when := int(time.Until(action.Date).Minutes())
		if when > 0 {
			if err := exec.Command("shutdown", "-P", strconv.Itoa(when)).Run(); err != nil {
				fmt.Printf("[ERROR]: could not initiate power off, reason: %v", err)
			}
		} else {
			if err := exec.Command("shutdown", "-P", "now").Run(); err != nil {
				fmt.Printf("[ERROR]: could not initiate shutdown, reason: %v", err)
			}
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent power off, reason: %v", err)
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

func (a *Agent) AgentCertificateHandler(msg jetstream.Msg) {

	data := openuem_nats.AgentCertificateData{}

	if err := json.Unmarshal(msg.Data(), &data); err != nil {
		log.Printf("[ERROR]: could not unmarshal agent certificate data, reason: %v\n", err)
		msg.Ack()
		return
	}

	wd := "/etc/openuem-agent"

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
