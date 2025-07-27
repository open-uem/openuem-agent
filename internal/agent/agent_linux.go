//go:build linux

package agent

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/apenella/go-ansible/v2/pkg/execute"
	"github.com/apenella/go-ansible/v2/pkg/execute/measure"
	results "github.com/apenella/go-ansible/v2/pkg/execute/result/json"
	"github.com/apenella/go-ansible/v2/pkg/execute/stdoutcallback"
	"github.com/apenella/go-ansible/v2/pkg/execute/workflow"
	galaxy "github.com/apenella/go-ansible/v2/pkg/galaxy/collection/install"
	"github.com/apenella/go-ansible/v2/pkg/playbook"
	"github.com/dgraph-io/badger/v4"
	"github.com/go-co-op/gocron/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/agent/rustdesk"
	rd "github.com/open-uem/openuem-agent/internal/commands/remote-desktop"
	"github.com/open-uem/openuem-agent/internal/commands/sftp"
	ansiblecfg "github.com/open-uem/openuem-ansible-config/ansible"
	openuem_utils "github.com/open-uem/utils"
	"gopkg.in/yaml.v3"
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
	if a.Config.SFTPPort != "" && !a.Config.SFTPDisabled {
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
	} else {
		log.Println("[INFO]: SFTP port is not set so SFTP server is not started!")
	}

	// Try to connect to NATS server and start a reconnect job if failed
	a.NATSConnection, err = openuem_nats.ConnectWithNATS(a.Config.NATSServers, a.Config.AgentCert, a.Config.AgentKey, a.Config.CACert)
	if err != nil {
		log.Printf("[ERROR]: %v", err)
		a.startNATSConnectJob()
		return
	}
	a.SubscribeToNATSSubjects()

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
			// Get remote config
			if err := a.GetRemoteConfig(); err != nil {
				log.Printf("[ERROR]: could not get remote config %v", err)
			}
			log.Println("[INFO]: remote config requested")

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
	a.startCheckForAnsibleProfilesJob()
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

				// Start the rest of tasks
				a.startReportJob()
				a.startPendingACKJob()
				a.startCheckForAnsibleProfilesJob()
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

func (a *Agent) StartRemoteDesktopSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.startvnc."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {

		// Instantiate new vnc server, but first try to check if certificates are there
		a.GetServerCertificate()
		if a.ServerCertPath == "" || a.ServerKeyPath == "" {
			log.Println("[ERROR]: Remote Desktop service requires a server certificate that it's not ready")
			return
		}

		v, err := rd.New(a.ServerCertPath, a.ServerKeyPath, "", a.Config.VNCProxyPort)
		if err != nil {
			log.Println("[ERROR]: could not get a Remote Desktop service")
			return
		}

		// Unmarshal data
		var rdConn openuem_nats.VNCConnection
		if err := json.Unmarshal(msg.Data, &rdConn); err != nil {
			log.Println("[ERROR]: could not unmarshall Remote Desktop connection")
			return
		}

		// Start Remote Desktop service
		a.RemoteDesktop = v
		v.Start(rdConn.PIN, rdConn.NotifyUser)

		if err := msg.Respond([]byte("Remote Desktop service started!")); err != nil {
			log.Printf("[ERROR]: could not respond to agent start vnc message, reason: %v\n", err)
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent start vnc, reason: %v", err)
	}
	return nil
}

func (a *Agent) StartRustDeskSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.rustdesk.start."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {

		rd := rustdesk.New()

		if err := rd.GetInstallationInfo(); err != nil {
			rustDeskRespond(msg, "", err.Error())
			return
		}

		if err := rd.Configure(msg.Data); err != nil {
			rustDeskRespond(msg, "", err.Error())
			return
		}

		if err := rd.LaunchRustDesk(); err != nil {
			rustDeskRespond(msg, "", err.Error())
			return
		}

		id, err := rd.GetRustDeskID()
		if err != nil {
			rustDeskRespond(msg, "", err.Error())
			return
		}

		// Send ID to the console
		rustDeskRespond(msg, id, "")
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to rustdesk start subject, reason: %v", err)
	}
	return nil
}

func (a *Agent) StopRustDeskSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.rustdesk.stop."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		if err := rustdesk.KillProcess(); err != nil {
			rustDeskRespond(msg, "", err.Error())
			return
		}
		rustDeskRespond(msg, "", "")
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to rustdesk stop subject, reason: %v", err)
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
				log.Printf("[ERROR]: could not initiate power off, reason: %v", err)
			}
		} else {
			if err := exec.Command("shutdown", "-r", "now").Run(); err != nil {
				log.Printf("[ERROR]: could not initiate shutdown, reason: %v", err)
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
				log.Printf("[ERROR]: could not initiate power off, reason: %v", err)
			}
		} else {
			if err := exec.Command("shutdown", "-P", "now").Run(); err != nil {
				log.Printf("[ERROR]: could not initiate shutdown, reason: %v", err)
			}
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent power off, reason: %v", err)
	}
	return nil
}

func (a *Agent) RescheduleAnsibleConfigureTask() {
	a.TaskScheduler.RemoveJob(a.WingetConfigureJob.ID())
	a.startCheckForAnsibleProfilesJob()
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
		a.Config.SFTPDisabled = config.SFTPDisabled
		a.Config.RemoteAssistanceDisabled = config.RemoteAssistanceDisabled

		// Should we re-schedule agent report?
		if a.Config.ExecuteTaskEveryXMinutes != SCHEDULETIME_5MIN {
			a.Config.ExecuteTaskEveryXMinutes = a.Config.DefaultFrequency
			a.RescheduleReportRunTask()
		}

		// Should we re-schedule ansible configure task?
		if config.WinGetFrequency != 0 {
			a.Config.WingetConfigureFrequency = config.WinGetFrequency
			a.RescheduleAnsibleConfigureTask()
		}

		if err := a.Config.WriteConfig(); err != nil {
			log.Fatalf("[FATAL]: could not write agent config: %v", err)
		}

		if err := a.Config.SetRestartRequiredFlag(); err != nil {
			log.Printf("[ERROR]: could not set restart required flag, reason: %v\n", err)
			return
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
		if err := msg.Ack(); err != nil {
			log.Printf("[ERROR]: could not ACK message, reason: %v", err)
			return
		}

		return
	}

	wd := "/etc/openuem-agent"

	if err := os.MkdirAll(filepath.Join(wd, "certificates"), 0660); err != nil {
		log.Printf("[ERROR]: could not create certificates folder, reason: %v\n", err)
		if err := msg.Ack(); err != nil {
			log.Printf("[ERROR]: could not ACK message, reason: %v", err)
		}
		return
	}

	keyPath := filepath.Join(wd, "certificates", "server.key")

	privateKey, err := x509.ParsePKCS1PrivateKey(data.PrivateKeyBytes)
	if err != nil {
		log.Printf("[ERROR]: could not get private key, reason: %v\n", err)
		if err := msg.Ack(); err != nil {
			log.Printf("[ERROR]: could not ACK message, reason: %v", err)
		}
		return
	}

	err = openuem_utils.SavePrivateKey(privateKey, keyPath)
	if err != nil {
		log.Printf("[ERROR]: could not save agent private key, reason: %v\n", err)
		if err := msg.Ack(); err != nil {
			log.Printf("[ERROR]: could not ACK message, reason: %v", err)
		}
		return
	}
	log.Printf("[INFO]: Agent private key saved in %s", keyPath)

	certPath := filepath.Join(wd, "certificates", "server.cer")
	err = openuem_utils.SaveCertificate(data.CertBytes, certPath)
	if err != nil {
		log.Printf("[ERROR]: could not save agent certificate, reason: %v\n", err)
		if err := msg.Ack(); err != nil {
			log.Printf("[ERROR]: could not ACK message, reason: %v", err)
		}
		return
	}
	log.Printf("[INFO]: Agent certificate saved in %s", certPath)

	if err := msg.Ack(); err != nil {
		log.Printf("[ERROR]: could not ACK message, reason: %v", err)
	}

	// Finally run a new report to inform that the certificate is ready
	r := a.RunReport()
	if r == nil {
		return
	}
}

func (a *Agent) GetServerCertificate() {

	cwd := "/etc/openuem-agent"

	serverCertPath := filepath.Join(cwd, "certificates", "server.cer")
	_, err := openuem_utils.ReadPEMCertificate(serverCertPath)
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

func (a *Agent) startCheckForAnsibleProfilesJob() error {
	var err error
	// Create task for running the agent

	if a.Config.WingetConfigureFrequency == 0 {
		a.Config.WingetConfigureFrequency = SCHEDULETIME_30MIN
	}

	a.WingetConfigureJob, err = a.TaskScheduler.NewJob(
		gocron.DurationJob(
			time.Duration(a.Config.WingetConfigureFrequency)*time.Minute,
		),
		gocron.NewTask(a.GetUnixConfigureProfiles),
	)
	if err != nil {
		log.Fatalf("[FATAL]: could not start the check for Ansible profiles job, reason: %v", err)
		return err
	}
	log.Printf("[INFO]: new check for Ansible profiles job has been scheduled every %d minutes", a.Config.WingetConfigureFrequency)
	return nil
}

func (a *Agent) GetUnixConfigureProfiles() {
	if a.Config.Debug {
		log.Println("[DEBUG]: running task Ansible profiles job")
	}

	profiles := []ProfileConfig{}

	profileRequest := openuem_nats.CfgProfiles{
		AgentID: a.Config.UUID,
	}

	if a.Config.Debug {
		log.Println("[DEBUG]: going to send a ansible.profile request")
	}

	data, err := json.Marshal(profileRequest)
	if err != nil {
		log.Printf("[ERROR]: could not marshal profile request, reason: %v", err)
	}

	if a.Config.Debug {
		log.Println("[DEBUG]: ansiblecfg.profile sending request")
	}

	msg, err := a.NATSConnection.Request("ansiblecfg.profiles", data, 5*time.Minute)
	if err != nil {
		log.Printf("[ERROR]: could not send request to agent worker, reason: %v", err)
	}

	if a.Config.Debug {
		log.Println("[DEBUG]: ansiblecfg.profile request sent")
		if msg.Data != nil {
			log.Println("[DEBUG]: received ansiblecfg.profile response")
		}
	}

	if err := yaml.Unmarshal(msg.Data, &profiles); err != nil {
		log.Printf("[ERROR]: could not unmarshal profiles response from agent worker, reason: %v", err)
	}

	if a.Config.Debug {
		log.Println("[DEBUG]: ansiblecfg.profile response unmarshalled")
	}

	if len(profiles) > 0 {
		if err := installCommunityGeneralCollection(); err != nil {
			log.Printf("[ERROR]: could not install ansible community general collection, reason: %v", err)
		} else {
			log.Println("[INFO]: ansible community general collection has been installed")
		}
	}

	for _, p := range profiles {
		if a.Config.Debug {
			log.Println("[DEBUG]: ansiblecfg.profile to be unmarshalled")
		}

		cfg, err := yaml.Marshal(p.AnsibleConfig)
		if err != nil {
			log.Printf("[ERROR]: could not marshal YAML file with Ansible configuration, reason: %v", err)
			continue
		}

		if a.Config.Debug {
			log.Println("[DEBUG]: we're going to apply the configuration")
		}

		if err := a.ApplyConfiguration(p.ProfileID, cfg); err != nil {
			log.Println("[ERROR]: could not apply YAML configuration file with Ansible")
			continue
		}
	}
}

func (a *Agent) ApplyConfiguration(profileID int, config []byte) error {
	var cfg []ansiblecfg.AnsiblePlaybook

	if err := yaml.Unmarshal(config, &cfg); err != nil {
		log.Printf("[ERROR]: could not unmarshall Ansible playbook folder %v", err)
		return err
	}

	ansibleFolder, err := CreatePlaybooksFolder()
	if err != nil {
		log.Printf("[ERROR]: could not create playbooks folder %v", err)
		return err
	}

	pbFile, err := os.CreateTemp(ansibleFolder, "*.yml")
	if err != nil {
		log.Printf("[ERROR]: could not create playbook file %v", err)
		return err
	}

	_, err = pbFile.WriteString("---\n\n")
	if err != nil {
		log.Printf("[ERROR]: could not write start of playbook to file %v", err)
		return err
	}

	_, err = pbFile.Write(config)
	if err != nil {
		log.Printf("[ERROR]: could not write playbook file %v", err)
		return err
	}

	if err := pbFile.Close(); err != nil {
		log.Printf("[ERROR]: could not close playbook file %v", err)
		return err
	}

	if !a.Config.Debug {
		defer func() {
			if err := os.Remove(pbFile.Name()); err != nil {
				log.Printf("[ERROR]: could not delete playbook file %v", err)
			}
		}()
	}

	log.Println("[INFO]: received a request to apply a configuration profile")

	errData := ""
	buff := new(bytes.Buffer)

	ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{
		Connection: "local",
		Inventory:  "127.0.0.1,",
	}

	playbookCmd := playbook.NewAnsiblePlaybookCmd(
		playbook.WithPlaybooks(pbFile.Name()),
		playbook.WithPlaybookOptions(ansiblePlaybookOptions),
	)

	exec := measure.NewExecutorTimeMeasurement(
		stdoutcallback.NewJSONStdoutCallbackExecute(
			execute.NewDefaultExecute(
				execute.WithCmd(playbookCmd),
				execute.WithErrorEnrich(playbook.NewAnsiblePlaybookErrorEnrich()),
				execute.WithWrite(io.Writer(buff)),
			),
		),
	)

	err = exec.Execute(context.TODO())
	if err != nil {
		generalError := err
		res, err := results.ParseJSONResultsStream(io.Reader(buff))
		if err == nil {
			errData = res.String()
		}
		if errData == "" {
			errData = generalError.Error()
		}
	} else {
		log.Println("[INFO]: ansible configuration has finished successfully")
	}

	// Report if application was successful or not
	if err := a.SendWinGetCfgProfileApplicationReport(profileID, a.Config.UUID, errData == "", errData); err != nil {
		log.Println("[ERROR]: could not report if profile was applied succesfully or no")
	}

	if err != nil {
		return errors.New(errData)
	}
	return nil
}

func CreatePlaybooksFolder() (string, error) {
	cwd, err := Getwd()
	if err != nil {
		log.Println("[ERROR]: could not get working directory")
		return "", errors.New("could not get working directory")
	}

	folder := filepath.Join(cwd, "ansible")
	return folder, os.MkdirAll(folder, 0660)
}

func installCommunityGeneralCollection() error {
	ansibleFolder, err := CreatePlaybooksFolder()
	if err != nil {
		log.Printf("[ERROR]: could not create playbooks folder %v", err)
		return err
	}

	pbFile, err := os.CreateTemp(ansibleFolder, "*.yml")
	if err != nil {
		log.Printf("[ERROR]: could not create playbook file %v", err)
		return err
	}

	_, err = pbFile.WriteString("---\n\ncollections:\n- name: community.general")
	if err != nil {
		log.Printf("[ERROR]: could not write start of playbook to file %v", err)
		return err
	}

	if err := pbFile.Close(); err != nil {
		log.Printf("[ERROR]: could not close playbook file %v", err)
		return err
	}

	galaxyInstallCollectionCmd := galaxy.NewAnsibleGalaxyCollectionInstallCmd(
		galaxy.WithGalaxyCollectionInstallOptions(&galaxy.AnsibleGalaxyCollectionInstallOptions{
			Force:            true,
			Upgrade:          true,
			RequirementsFile: pbFile.Name(),
		}),
	)

	galaxyInstallCollectionExec := execute.NewDefaultExecute(
		execute.WithCmd(galaxyInstallCollectionCmd),
	)

	return workflow.NewWorkflowExecute(galaxyInstallCollectionExec).WithTrace().Execute(context.TODO())
}

func rustDeskRespond(msg *nats.Msg, id string, errMessage string) {
	result := openuem_nats.RustDeskResult{
		RustDeskID: id,
		Error:      errMessage,
	}

	data, err := json.Marshal(result)
	if err != nil {
		log.Printf("[ERROR]: could not marshal RustDesk response, reason: %v\n", err)
	}

	if err := msg.Respond(data); err != nil {
		log.Printf("[ERROR]: could not respond to agent rustdesk start message, reason: %v\n", err)
		return
	}
}
