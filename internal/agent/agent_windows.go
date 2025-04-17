//go:build windows

package agent

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
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
	rd "github.com/open-uem/openuem-agent/internal/commands/remote-desktop"
	"github.com/open-uem/openuem-agent/internal/commands/report"
	"github.com/open-uem/openuem-agent/internal/commands/sftp"
	openuem_utils "github.com/open-uem/utils"
	"github.com/open-uem/wingetcfg/wingetcfg"
	"golang.org/x/mod/semver"
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
