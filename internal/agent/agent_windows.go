//go:build windows

package agent

import (
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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-co-op/gocron/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/agent/dsc"
	"github.com/open-uem/openuem-agent/internal/commands/deploy"
	rd "github.com/open-uem/openuem-agent/internal/commands/remote-desktop"
	"github.com/open-uem/openuem-agent/internal/commands/report"
	"github.com/open-uem/openuem-agent/internal/commands/sftp"
	openuem_utils "github.com/open-uem/utils"
	"github.com/open-uem/wingetcfg/wingetcfg"
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
	a.startCheckForWinGetProfilesJob()
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
				a.startCheckForWinGetProfilesJob()
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

		// Instantiate new remote desktop service, but first try to check if certificates are there
		a.GetServerCertificate()
		if a.ServerCertPath == "" || a.ServerKeyPath == "" {
			log.Println("[ERROR]: Remote Desktop requires a server certificate that it's not ready")
			return
		}

		v, err := rd.New(a.ServerCertPath, a.ServerKeyPath, sid, a.Config.VNCProxyPort)
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

		// Start Remote Desktop server
		a.RemoteDesktop = v
		v.Start(rdConn.PIN, rdConn.NotifyUser)

		if err := msg.Respond([]byte("Remote Desktop service started!")); err != nil {
			log.Printf("[ERROR]: could not respond to agent start remote desktop message, reason: %v\n", err)
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent start remote desktop, reason: %v", err)
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

		when := int(time.Until(action.Date).Seconds())
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

		when := int(time.Until(action.Date).Seconds())
		if when > 0 {
			if err := exec.Command("cmd", "/C", "shutdown", "/s", "/t", strconv.Itoa(when)).Run(); err != nil {
				log.Printf("[ERROR]: could not initiate power off, reason: %v", err)
			}
		} else {
			if err := exec.Command("cmd", "/C", "shutdown", "/s").Run(); err != nil {
				log.Printf("[ERROR]: could not initiate shutdown, reason: %v", err)
			}
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent power off, reason: %v", err)
	}
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

func (a *Agent) GetWingetConfigureProfiles() {
	if a.Config.Debug {
		log.Println("[DEBUG]: running task WinGet profiles job")
	}

	profiles := []openuem_nats.ProfileConfig{}

	profileRequest := openuem_nats.CfgProfiles{
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
		if err := a.Config.SetRestartRequiredFlag(); err != nil {
			log.Printf("[ERROR]: could not set restart required flag, reason: %v\n", err)
			return
		}
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

		cfg, err := yaml.Marshal(p.WinGetConfig)
		if err != nil {
			log.Printf("[ERROR]: could not marshal YAML file with winget configuration, reason: %v", err)
			continue
		}

		if a.Config.Debug {
			log.Println("[DEBUG]: we're going to apply the configuration")
		}

		// Read task control file
		cwd, err := openuem_utils.GetWd()
		if err != nil {
			log.Printf("[ERROR]: could not get working directory, reason %v", err)
			return
		}
		taskControlPath := filepath.Join(cwd, "powershell", "tasks.json")
		taskControl, err := dsc.ReadTaskControlFile(taskControlPath)

		if err != nil {
			log.Printf("[ERROR]: tasks control file is not available, reason %v", err)
			return
		}

		ansibleErrData, err := a.ApplyConfiguration(p.ProfileID, cfg, p.Exclusions, p.Deployments, taskControl, taskControlPath)
		if err != nil {
			log.Printf("[ERROR]: could not apply YAML configuration file with winget, reason: %v", err)
			continue
		}

		// Netbird tasks
		errData := ""
		nbErrData := a.ApplyNetBirdConfiguration(p, taskControl, taskControlPath)
		if nbErrData != nil {
			log.Println("[ERROR]: could not apply Netbird configuration file")
			errData = strings.Join([]string{ansibleErrData, nbErrData.Error()}, ",")
		}

		// Report if application was successful or not
		if err := a.SendProfileApplicationReport(p.ProfileID, a.Config.UUID, errData == "", errData); err != nil {
			log.Println("[ERROR]: could not report if profile was applied succesfully or no")
		}

		if err := a.SendProfileApplicationReport(p.ProfileID, a.Config.UUID, errData == "", errData); err != nil {
			log.Println("[ERROR]: could not report if profile was applied succesfully or not")
		}
	}
}

func (a *Agent) ApplyConfiguration(profileID int, config []byte, exclusions, deployments []string, taskControl *dsc.TaskControl, taskControlPath string) (string, error) {
	var cfg wingetcfg.WinGetCfg

	// Unmarshall profile
	if err := yaml.Unmarshal(config, &cfg); err != nil {
		return "", err
	}

	ID := strconv.Itoa(profileID)
	if taskControl.ProfilesRunning == nil {
		taskControl.ProfilesRunning = map[string]time.Time{
			ID: time.Now(),
		}
	} else {
		when, ok := taskControl.ProfilesRunning[ID]
		if !ok {
			taskControl.ProfilesRunning[ID] = time.Now()
		} else {
			// Clear stalled profile for more than 24 hours
			if time.Now().After(when.Add(24 * time.Hour)) {
				log.Printf("[INFO]: found previous task %s that hasn't be re-run for more than 24 hours", ID)
				taskControl.ProfilesRunning[ID] = time.Now()
			} else {
				log.Printf("[INFO]: previous profile %s is marked as running, not relaunching, ", ID)
				return "", nil
			}
		}
	}
	if err := dsc.SaveTaskControl(taskControlPath, taskControl); err != nil {
		log.Printf("[ERROR]: could not save new profile %s running, reason: %v", ID, err)
		return "", err
	}

	defer func() {
		delete(taskControl.ProfilesRunning, ID)
		if err := dsc.SaveTaskControl(taskControlPath, taskControl); err != nil {
			log.Printf("[ERROR]: could not remove profile %s from running, reason: %v", ID, err)
			return
		}
	}()

	// Run tasks defined in the profile and report if profile was applied successfully
	errProfile := a.RunTasks(cfg, profileID, taskControlPath, taskControl)
	errData := ""
	if errProfile != nil {
		errData = errProfile.Error()
	}

	return errData, nil
}

func (a *Agent) CheckIfCfgPackagesInstalled(cfg wingetcfg.WinGetCfg, installed string) {
	for _, r := range cfg.Properties.Resources {
		if r.Resource == wingetcfg.WinGetPackageResource {
			packageID := r.Settings["id"].(string)
			packageName := r.Directives.Description
			if r.Settings["Ensure"].(string) == "Present" {
				if strings.Contains(installed, packageID) {
					if err := a.SendWinGetCfgDeploymentReport(packageID, packageName, "install"); err != nil {
						log.Printf("[ERROR]: could not send WinGetCfg deployment report, reason: %v", err)
						continue
					}
				}
			} else {
				if !strings.Contains(installed, packageID) {
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

func (a *Agent) RescheduleWingetConfigureTask() {
	a.TaskScheduler.RemoveJob(a.WingetConfigureJob.ID())
	a.startCheckForWinGetProfilesJob()
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

		if a.Config.SFTPDisabled != config.SFTPDisabled {
			if err := a.Config.SetRestartRequiredFlag(); err != nil {
				log.Printf("[ERROR]: could not set restart required flag, reason: %v\n", err)
				return
			}
		}
		a.Config.SFTPDisabled = config.SFTPDisabled

		a.Config.RemoteAssistanceDisabled = config.RemoteAssistanceDisabled

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

func (a *Agent) AgentCertificateHandler(msg jetstream.Msg) {

	data := openuem_nats.AgentCertificateData{}

	if err := json.Unmarshal(msg.Data(), &data); err != nil {
		log.Printf("[ERROR]: could not unmarshal agent certificate data, reason: %v\n", err)
		if err := msg.Ack(); err != nil {
			log.Printf("[ERROR]: could not ACK message, reason: %v", err)
		}
		return
	}

	wd, err := openuem_utils.GetWd()
	if err != nil {
		log.Printf("[ERROR]: could not get working directory, reason: %v\n", err)
		if err := msg.Ack(); err != nil {
			log.Printf("[ERROR]: could not ACK message, reason: %v", err)
		}
		return
	}

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
	log.Printf("[INFO]: Agent certificate saved in %s", keyPath)

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

func (a *Agent) ExecutePowerShellScript(script string) error {
	if script != "" {
		file, err := os.CreateTemp(os.TempDir(), "*.ps1")
		if err != nil {
			fmt.Printf("[ERROR]: could not create temp ps1 file, reason: %v", err)
			return errors.New("could not create temp ps1 file")
		} else {
			defer func() {
				if err := file.Close(); err != nil {
					fmt.Printf("[ERROR]: could not close the file, maybe it was closed earlier, reason: %v", err)
				}
			}()
			if _, err := file.Write([]byte(script)); err != nil {
				fmt.Printf("[ERROR]: could not execute write on temp ps1 file, reason: %v", err)
				return errors.New("could not execute write on temp ps1 file")
			}
			if err := file.Close(); err != nil {
				fmt.Printf("[ERROR]: could not close temp ps1 file, reason: %v", err)
				return errors.New("could not close temp ps1 file")
			}

			// Get current Execution-Policy
			out, err := exec.Command("PowerShell", "-command", "Get-ExecutionPolicy -Scope CurrentUser").CombinedOutput()
			if err != nil {
				fmt.Printf("[ERROR]: could not get current Powershell execution policy, reason: %v, %s", err, string(out))
				return errors.New("could not get current Powershell execution policy")
			}
			currentExecutionPolicy := strings.TrimSpace(string(out))

			// Set ExecutionPolicy temporarily to RemoteSigned
			out, err = exec.Command("PowerShell", "-command", "Set-ExecutionPolicy RemoteSigned -Scope CurrentUser").CombinedOutput()
			if err != nil {
				fmt.Printf("[ERROR]: could not set Powershell execution policy to RemoteSigned temporarily, reason: %v, %s", err, string(out))
				return errors.New("could not set Powershell execution policy to RemoteSigned temporarily")
			}
			defer func() {
				// Revert back to previous ExecutionPolicy
				out, err = exec.Command("PowerShell", "-command", fmt.Sprintf("Set-ExecutionPolicy %s -Scope CurrentUser", currentExecutionPolicy)).CombinedOutput()
				if err != nil {
					fmt.Printf("[ERROR]: could not revert the Powershell execution policy to RemoteSigned temporarily, reason: %v, %s", err, string(out))
				}
			}()

			if out, err := exec.Command("PowerShell", "-File", file.Name()).CombinedOutput(); err != nil {
				fmt.Printf("[ERROR]: could not execute powershell script, reason: %v, %s", err, string(out))
				return errors.New("could not execute powershell script")
			}
			if a.Config.Debug {
				log.Printf("[DEBUG]: a script should have run: PowerShell -File %s", file.Name())
			}

			if err := os.Remove(file.Name()); err != nil {
				fmt.Printf("[ERROR]: could not remove temp ps1 file, reason: %v", err)
			}
		}
	}

	return nil
}

func (a *Agent) RunTasks(cfg wingetcfg.WinGetCfg, profileID int, taskControlPath string, taskControl *dsc.TaskControl) error {
	errData := ""
	for _, resource := range cfg.Properties.Resources {
		var err error
		switch resource.Resource {
		case wingetcfg.OpenUEMPowershell:
			err = a.PowershellTask(resource, taskControlPath, taskControl)
		case wingetcfg.WinGetLocalGroupResource:
			err = a.LocalGroupTask(resource, taskControlPath, taskControl)
		case wingetcfg.WinGetLocalUserResource:
			err = a.LocalUserTask(resource, taskControlPath, taskControl)
		case wingetcfg.WinGetMSIPackageResource:
			err = a.MSIPackageTask(resource, taskControlPath, taskControl)
		case wingetcfg.WinGetPackageResource:
			err = a.PackageManagementTask(resource, taskControlPath, taskControl)
		case wingetcfg.WinGetRegistryResource:
			err = a.RegistryTask(resource, taskControlPath, taskControl)
		}

		if err != nil {
			when := time.Now().Local().Format(time.RFC822)
			if errData != "" {
				errData += fmt.Sprintf(", [%s] %s", when, err.Error())
			} else {
				errData = fmt.Sprintf("[%s] %s", when, err.Error())
			}
		}
	}

	if errData != "" {
		return errors.New(errData)
	}

	return nil
}

func (a *Agent) PackageManagementTask(r *wingetcfg.WinGetResource, taskControlPath string, t *dsc.TaskControl) error {

	packageName := r.Directives.Description

	ensure, err := getEnsureKey(r)
	if err != nil {
		return err
	}

	key, ok := r.Settings["id"]
	if !ok {
		return errors.New("could not find the id key for a package management task")
	}
	packageID := key.(string)

	version := ""
	key, ok = r.Settings["version"]
	if ok {
		version = key.(string)
	}

	keepUpdated := false
	key, ok = r.Settings["uselatest"]
	if ok {
		keepUpdated = key.(bool)
	}

	if ensure == "Present" {
		taskAlreadySuccessful := slices.Contains(t.Success, r.ID)

		// if a package has to be kept udpdated but hasn't passed 24 hours since the last execution skip
		if keepUpdated {
			timeExecuted, ok := t.Executed[r.ID]
			if ok {
				hasPassed24h := time.Now().After(timeExecuted.Add(24 * time.Hour))
				if !hasPassed24h {
					return nil
				}
			}
		}

		if !taskAlreadySuccessful {
			if err := deploy.InstallPackage(packageID, version, keepUpdated, a.Config.Debug); err != nil {
				if keepUpdated && strings.Contains(err.Error(), "0x8a15002b") {
					if t.Executed == nil {
						t.Executed = map[string]time.Time{
							r.ID: time.Now(),
						}
					} else {
						t.Executed[r.ID] = time.Now()
					}
					return dsc.SaveTaskControl(taskControlPath, t)
				}
				return err
			}
			if err := a.SendWinGetCfgDeploymentReport(packageID, packageName, "install"); err != nil {
				log.Printf("[ERROR]: could not send WinGetCfg deployment report, reason: %v", err)
				return err
			}

			// if package must not be kept updated, mark the task as successful
			if !keepUpdated {
				return dsc.SetTaskAsSuccessfull(r.ID, taskControlPath, t)
			}
		}
	} else {
		taskAlreadySuccessful := slices.Contains(t.Success, r.ID)

		if !taskAlreadySuccessful {
			if err := deploy.UninstallPackage(packageID); err != nil {
				return err
			}
			if err := a.SendWinGetCfgDeploymentReport(packageID, packageName, "uninstall"); err != nil {
				log.Printf("[ERROR]: could not send WinGetCfg deployment report, reason: %v", err)
				return err
			}

			return dsc.SetTaskAsSuccessfull(r.ID, taskControlPath, t)
		}
	}

	return nil
}

func (a *Agent) RegistryTask(r *wingetcfg.WinGetResource, taskControlPath string, t *dsc.TaskControl) error {
	ensure, err := getEnsureKey(r)
	if err != nil {
		return err
	}

	key, err := getStringKey(r, "Key", -1, true)
	if err != nil {
		return err
	}

	force, err := getBoolKey(r, "Force", false)
	if err != nil {
		return err
	}

	valueName, err := getStringKey(r, "ValueName", -1, false)
	if err != nil {
		return err
	}

	valueData, err := getStringKey(r, "ValueData", -1, false)
	if err != nil {
		return err
	}

	propertyType, err := getStringKey(r, "ValueType", -1, false)
	if err != nil {
		return err
	}

	hex, err := getBoolKey(r, "Hex", false)
	if err != nil {
		return err
	}

	taskAlreadySuccessful := slices.Contains(t.Success, r.ID)
	if !taskAlreadySuccessful {
		if ensure == "Present" {

			if valueName == "" {
				if valueData != "" {
					if err := dsc.UpdateRegistryKeyDefaultValue(key, valueData); err != nil {
						log.Printf("[ERROR]: could not update registry %s default value, reason: %v", key, err)
						return fmt.Errorf("could not update registry %s default value, reason: %v", key, err)
					}
					log.Printf("[INFO]: registry key default value %s has been updated", key)
				} else {
					if err := dsc.AddRegistryKey(key, force); err != nil {
						log.Printf("[ERROR]: could not add registry key %s, reason: %v", key, err)
						return fmt.Errorf("could not add registry key %s, reason: %v", key, err)
					}
					log.Printf("[INFO]: registry key %s has been added", key)
				}

			} else {
				if !wingetcfg.IsValidRegistryValueType(propertyType) {
					return fmt.Errorf("could not add registry value key %s, reason: property type %s is not valid", valueName, propertyType)
				}

				if err := dsc.AddOrEditRegistryValue(key, valueName, propertyType, valueData, hex, force); err != nil {
					log.Printf("[ERROR]: could not add registry value key %s, reason: %v", valueName, err)
					return fmt.Errorf("could not add registry value key %s, reason: %v", valueName, err)
				}

				log.Printf("[INFO]: registry key value %s has been added", valueName)
			}

		} else {
			force, err := getBoolKey(r, "Force", false)
			if err != nil {
				return err
			}

			valueName, err := getStringKey(r, "ValueName", -1, false)
			if err != nil {
				return err
			}

			if valueName == "" {
				if err := dsc.RemoveRegistryKey(key, force); err != nil {
					log.Printf("[ERROR]: could not remove registry key %s, reason: %v", key, err)
					return fmt.Errorf("could not remove registry key %s, reason: %v", key, err)
				}
				log.Printf("[INFO]: registry key %s has been removed", key)
			} else {
				if err := dsc.RemoveRegistryKeyValue(key, valueName); err != nil {
					log.Printf("[ERROR]: could not remove registry key value %s, reason: %v", valueName, err)
					return fmt.Errorf("could not remove registry key value %s, reason: %v", valueName, err)
				}
				log.Printf("[INFO]: registry key value %s has been removed", valueName)
			}
		}

		return dsc.SetTaskAsSuccessfull(r.ID, taskControlPath, t)
	}

	return nil
}

func (a *Agent) LocalUserTask(r *wingetcfg.WinGetResource, taskControlPath string, t *dsc.TaskControl) error {
	ensure, err := getEnsureKey(r)
	if err != nil {
		return err
	}

	username, err := getStringKey(r, "UserName", 20, true)
	if err != nil {
		return err
	}

	taskAlreadySuccessful := slices.Contains(t.Success, r.ID)

	if !taskAlreadySuccessful {

		// Create user
		if ensure == "Present" {

			// Set password if exist
			password, err := getStringKey(r, "Password", -1, false)
			if err != nil {
				return err
			}

			// Options - Comment
			comment, err := getStringKey(r, "Description", 48, false)
			if err != nil {
				return err
			}

			// Options - FullName
			fullName, err := getStringKey(r, "FullName", 48, false)
			if err != nil {
				return err
			}

			// Options - Disabled
			disabled, err := getBoolKey(r, "Disabled", false)
			if err != nil {
				return err
			}

			// Options - Password Change Not Allowed
			passwordChangeNotAllowed, err := getBoolKey(r, "PasswordChangeNotAllowed", false)
			if err != nil {
				return err
			}

			// Options - Password never expires
			passwordNeverExpires, err := getBoolKey(r, "PasswordNeverExpires", false)
			if err != nil {
				return err
			}

			// Options - Force password change at logon
			changePasswordAtLogon, err := getBoolKey(r, "PasswordChangeRequired", false)
			if err != nil {
				return err
			}

			if err := dsc.CreateLocalUser(username, password, comment, fullName, disabled, passwordChangeNotAllowed, passwordNeverExpires, changePasswordAtLogon); err != nil {
				log.Printf("[ERROR]: could not create the local user, reason: %v", err)
				return fmt.Errorf("could not create the local user, reason: %v", err)
			}
			log.Printf("[INFO]: the local user %s has been added", username)

		} else {
			if err := dsc.DeleteLocalUser(username); err != nil {
				log.Printf("[ERROR]: could not remove the local user, reason: %v", err)
				return fmt.Errorf("could not remove the local user, reason: %v", err)
			}
			log.Printf("[INFO]: the local user %s has been deleted", username)
		}

		return dsc.SetTaskAsSuccessfull(r.ID, taskControlPath, t)
	}

	return nil
}

func (a *Agent) LocalGroupTask(r *wingetcfg.WinGetResource, taskControlPath string, t *dsc.TaskControl) error {
	ensure, err := getEnsureKey(r)
	if err != nil {
		return err
	}

	groupName, err := getStringKey(r, "GroupName", 64, true)
	if err != nil {
		return err
	}

	description, err := getStringKey(r, "Description", 256, false)
	if err != nil {
		return err
	}

	taskAlreadySuccessful := slices.Contains(t.Success, r.ID)

	if !taskAlreadySuccessful {

		if ensure == "Present" {
			members, err := getCommaSeparatedStringKey(r, "Members", false)
			if err != nil {
				return err
			}

			membersToInclude, err := getCommaSeparatedStringKey(r, "MembersToInclude", false)
			if err != nil {
				return err
			}

			membersToExclude, err := getCommaSeparatedStringKey(r, "MembersToExclude", false)
			if err != nil {
				return err
			}

			if members == "" && membersToInclude == "" && membersToExclude == "" {
				if err := dsc.CreateLocalGroup(groupName, description); err != nil {
					log.Printf("[ERROR]: could not create local group, reason: %v", err)
					return fmt.Errorf("could not create local group, reason: %v", err)
				}
				log.Printf("[INFO]: the local group %s has been added", groupName)
				return dsc.SetTaskAsSuccessfull(r.ID, taskControlPath, t)
			}

			if members != "" {
				if !dsc.ExistsGroup(groupName) {
					if err := dsc.CreateLocalGroup(groupName, description); err != nil {
						log.Printf("[ERROR]: could not create local group, reason: %v", err)
						return fmt.Errorf("could not create local group, reason: %v", err)
					}
					log.Printf("[INFO]: the local group %s has been added", groupName)
				}

				if err := dsc.AddMembersToLocalGroup(groupName, members); err != nil {
					log.Printf("[ERROR]: could not add members to local group, reason: %v", err)
					return fmt.Errorf("could not add members to local group, reason: %v", err)
				}
				log.Printf("[INFO]: members have been added to local group %s", groupName)
				return dsc.SetTaskAsSuccessfull(r.ID, taskControlPath, t)

			}

			if membersToInclude != "" {
				if err := dsc.AddMembersToLocalGroup(groupName, membersToInclude); err != nil {
					log.Printf("[ERROR]: could not add members to local group, reason: %v", err)
					return fmt.Errorf("could not add members to local group, reason: %v", err)
				}
				log.Printf("[INFO]: members have been added to local group %s", groupName)
				return dsc.SetTaskAsSuccessfull(r.ID, taskControlPath, t)
			}

			if membersToExclude != "" {
				if err := dsc.RemoveMembersFromLocalGroup(groupName, membersToExclude); err != nil {
					log.Printf("[ERROR]: could not exclude members from local group, reason: %v", err)
					return fmt.Errorf("could not exclude members from local group, reason: %v", err)
				}
				log.Printf("[INFO]: members have been removed from local group %s", groupName)
				return dsc.SetTaskAsSuccessfull(r.ID, taskControlPath, t)
			}

		} else {
			if err := dsc.RemoveLocalGroup(groupName); err != nil {
				log.Printf("[ERROR]: could not delete local group, reason: %v", err)
				return fmt.Errorf("could not delete local group, reason: %v", err)
			}
			log.Printf("[INFO]: the local group %s has been deleted", groupName)
			return dsc.SetTaskAsSuccessfull(r.ID, taskControlPath, t)
		}
	}

	return nil
}

func (a *Agent) MSIPackageTask(r *wingetcfg.WinGetResource, taskControlPath string, t *dsc.TaskControl) error {
	ensure, err := getEnsureKey(r)
	if err != nil {
		return err
	}

	path, err := getStringKey(r, "Path", -1, true)
	if err != nil {
		return err
	}

	arguments, err := getStringKey(r, "Arguments", -1, false)
	if err != nil {
		return err
	}

	logPath, err := getStringKey(r, "LogPath", -1, false)
	if err != nil {
		return err
	}

	taskAlreadySuccessful := slices.Contains(t.Success, r.ID)

	if !taskAlreadySuccessful {
		if ensure == "Present" {
			if err := dsc.InstallMSIPackage(path, arguments, logPath); err != nil {
				log.Printf("[ERROR]: could not install MSI package reason: %v", err)
				return fmt.Errorf("could not install MSI package reason: %v", err)
			}
			log.Printf("[INFO]: MSI package has been installed from %s", path)
			return dsc.SetTaskAsSuccessfull(r.ID, taskControlPath, t)
		} else {
			if err := dsc.UninstallMSIPackage(path, arguments, logPath); err != nil {
				log.Printf("[ERROR]: could not uninstall MSI package reason: %v", err)
				return fmt.Errorf("could not uninstall MSI package reason: %v", err)
			}
			log.Printf("[INFO]: MSI package %s has been uninstalled", path)
			return dsc.SetTaskAsSuccessfull(r.ID, taskControlPath, t)
		}
	}

	return nil

}

type PowerShellTask struct {
	ID        string
	Name      string
	Script    string
	RunConfig string
}

func (a *Agent) PowershellTask(r *wingetcfg.WinGetResource, taskControlPath string, t *dsc.TaskControl) error {
	task := PowerShellTask{}

	script, ok := r.Settings["Script"]
	if ok {
		name, ok := r.Settings["Name"]
		if ok {
			id, ok := r.Settings["ID"]
			if ok {
				scriptRun, ok := r.Settings["ScriptRun"]
				task.ID = id.(string)
				task.Name = name.(string)
				task.Script = script.(string)

				if ok {
					task.RunConfig = scriptRun.(string)
				} else {
					task.RunConfig = "once"
				}
			} else {
				log.Println("[ERROR]: could not find ID key in task's settings")
				return errors.New("could not find ID key in task's settings")
			}
		} else {
			log.Println("[ERROR]: could not find Name key in task's settings")
			return errors.New("could not find Name key in task's settings")
		}
	} else {
		log.Println("[ERROR]: could not find Script key in task's settings")
		return errors.New("could not find script key in task's settings")
	}

	taskAlreadySuccessful := slices.Contains(t.Success, task.ID)
	if task.RunConfig == "once" {
		if !taskAlreadySuccessful {
			scriptsRun := strings.Split(a.Config.ScriptsRun, ",")
			if !slices.Contains(scriptsRun, task.ID) {
				if err := a.ExecutePowerShellScript(task.Script); err != nil {
					log.Printf("[ERROR]: errors were found running PowerShell, reason: %v", err)
					return fmt.Errorf("errors were found running PowerShell, reason: %v", err)
				}
			}
			log.Printf("[INFO]: powershell script %s run successfully", task.Name)
			return dsc.SetTaskAsSuccessfull(task.ID, taskControlPath, t)
		}
	} else {
		if err := a.ExecutePowerShellScript(task.Script); err != nil {
			log.Printf("[ERROR]: errors were found running PowerShell, reason: %v", err)
			return fmt.Errorf("errors were found running PowerShell, reason: %v", err)
		}
	}

	return nil
}

func getEnsureKey(r *wingetcfg.WinGetResource) (string, error) {
	value, ok := r.Settings["Ensure"].(string)
	if !ok {
		return "", errors.New("could not find the Ensure key")
	}
	if value != "Present" && value != "Absent" {
		return "", errors.New("unexpected Ensure key: " + value)
	}

	return value, nil
}

func getStringKey(r *wingetcfg.WinGetResource, key string, maxLength int, required bool) (string, error) {
	v, ok := r.Settings[key]
	if !ok {
		if required {
			return "", fmt.Errorf("%s is empty and is required", key)
		}
		return "", nil
	}

	value := v.(string)

	if maxLength > 0 && len(value) > maxLength {
		return "", fmt.Errorf("%s exceeds the %d character limit", key, maxLength)
	}

	return value, nil
}

func getBoolKey(r *wingetcfg.WinGetResource, key string, required bool) (bool, error) {
	v, ok := r.Settings[key]
	if !ok {
		if required {
			return false, fmt.Errorf("%s is empty and is required", key)
		}
		return false, nil
	}

	value := v.(bool)
	if !ok {
		return false, fmt.Errorf("could not find the %s key", key)
	}

	return value, nil
}

func getCommaSeparatedStringKey(r *wingetcfg.WinGetResource, key string, required bool) (string, error) {
	v, ok := r.Settings[key]
	if !ok {
		if required {
			return "", fmt.Errorf("%s is empty and is required", key)
		}
		return "", nil
	}

	values := strings.Split(v.(string), ";")

	csValues := []string{}
	for _, v := range values {
		csValues = append(csValues, fmt.Sprintf("'%s'", v))
	}

	return strings.Join(csValues, ", "), nil
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
