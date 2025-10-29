//go:build darwin

package rustdesk

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	"github.com/pelletier/go-toml/v2"
	"github.com/shirou/gopsutil/v3/process"
)

func (cfg *RustDeskConfig) Configure(config []byte) error {
	// Unmarshal configuration data sent by OpenUEM
	var rdConfig nats.RustDesk
	if err := json.Unmarshal(config, &rdConfig); err != nil {
		log.Println("[ERROR]: could not unmarshall RustDesk configuration")
		return err
	}

	// Inform in logs that no server settings have been received
	if rdConfig.CustomRendezVousServer == "" &&
		rdConfig.RelayServer == "" &&
		rdConfig.Key == "" &&
		rdConfig.APIServer == "" &&
		!rdConfig.DirectIPAccess {
		log.Println("[INFO]: no RustDesk server settings have been found for tenant, using RustDesk's default settings")
	}

	// Configuration file location
	configPath := "/System/Volumes/Data/private/var/root/Library/Preferences/com.carriez.RustDesk"
	configFile := filepath.Join(configPath, "RustDesk2.toml")

	// Create TOML file with new config
	cfgTOML := RustDeskOptions{
		Optional: RustDeskOptionsEntries{
			CustomRendezVousServer: rdConfig.CustomRendezVousServer,
			RelayServer:            rdConfig.RelayServer,
			Key:                    rdConfig.Key,
			ApiServer:              rdConfig.APIServer,
		},
	}

	if rdConfig.DirectIPAccess {
		cfgTOML.Optional.UseDirectIPAccess = "Y"
	}

	if rdConfig.Whitelist != "" {
		cfgTOML.Optional.Whitelist = rdConfig.Whitelist
	}

	rdTOML, err := toml.Marshal(cfgTOML)
	if err != nil {
		log.Printf("[ERROR]: could not marshall TOML file for RustDesk configuration, reason: %v", err)
		return err
	}

	// Check if configuration file exists, if exists create a backup
	if _, err := os.Stat(configFile); err == nil {
		if err := CopyFile(configFile, configFile+".bak"); err != nil {
			return err
		}
	} else {
		// Check if configuration path exists, if not create path
		if err := os.MkdirAll(configPath, 0644); err != nil {
			log.Printf("[ERROR]: could not create directory file for RustDesk configuration, reason: %v", err)
			return err
		}
	}

	// Write the new configuration file for RustDesk
	if err := os.WriteFile(configFile, rdTOML, 0600); err != nil {
		log.Printf("[ERROR]: could not create TOML file for RustDesk configuration, reason: %v", err)
		return err
	}

	// Restart RustDeskService
	if err := RestartRustDeskService(cfg.User.Username); err != nil {
		log.Printf("[ERROR]: could not start RustDesk service, reason: %v", err)
		return err
	}

	return nil
}

func (cfg *RustDeskConfig) GetInstallationInfo() error {
	rdUser, err := getRustDeskUserInfo()
	if err != nil {
		return err
	}
	cfg.User = rdUser

	binPath := "/Applications/RustDesk.app/Contents/MacOS/RustDesk"

	if _, err := os.Stat(binPath); err == nil {
		cfg.Binary = binPath
		cfg.GetIDArgs = []string{"--get-id"}
	}

	return nil
}

func (cfg *RustDeskConfig) LaunchRustDesk() error {
	return runtime.RunAsUserInBackground(cfg.User.Username, cfg.Binary, cfg.LaunchArgs, true)
}

func (cfg *RustDeskConfig) GetRustDeskID() (string, error) {
	// Get RustDesk ID
	out, err := runtime.RunAsUserWithOutput(cfg.User.Username, cfg.Binary, cfg.GetIDArgs, true)
	if err != nil {
		log.Printf("[ERROR]: could not get RustDesk ID, reason: %v", err)
		return "", err
	}

	id := strings.TrimSpace(string(out))
	_, err = strconv.Atoi(id)
	if err != nil {
		log.Printf("[ERROR]: RustDesk ID is not a number, reason: %v", err)
		return "", err
	}

	return id, nil
}

func getRustDeskUserInfo() (*RustDeskUser, error) {
	rdUser := RustDeskUser{}

	// Get current user logged in, uid, gid and home user
	username, err := runtime.GetLoggedInUser()
	if err != nil {
		log.Println("[ERROR]: could not get logged in user")
		return nil, err
	}
	rdUser.Username = username

	u, err := user.Lookup(username)
	if err != nil {
		log.Println("[ERROR]: could not find user information")
		return nil, err
	}
	rdUser.Home = u.HomeDir

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		log.Println("[ERROR]: could not get UID of logged in user")
		return nil, err
	}
	rdUser.Uid = uid

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		log.Println("[ERROR]: could not get GID of logged in user")
		return nil, err
	}
	rdUser.Gid = gid

	return &rdUser, nil
}

func (cfg *RustDeskConfig) KillRustDeskProcess() error {
	processes, err := process.Processes()
	if err != nil {
		return err
	}
	for _, p := range processes {
		n, err := p.Name()
		if err != nil {
			return err
		}
		if n == "rustdesk" {
			if err := p.Kill(); err != nil {
				log.Println("[ERROR]: could not kill RustDesk process ")
			}
		}
	}

	// Restart RustDeskService
	if err := RestartRustDeskService(cfg.User.Username); err != nil {
		log.Printf("[ERROR]: could not start RustDesk service, reason: %v", err)
		return err
	}

	return nil
}

func (cfg *RustDeskConfig) ConfigRollBack() error {

	// Configuration file location
	configPath := "/System/Volumes/Data/private/var/root/Library/Preferences/com.carriez.RustDesk"
	configFile := filepath.Join(configPath, "RustDesk2.toml")

	// Check if configuration backup exists, if exists rename the file
	if _, err := os.Stat(configFile + ".bak"); err == nil {
		if err := os.Rename(configFile+".bak", configFile); err != nil {
			return err
		}
	}

	// Restart RustDeskService
	if err := RestartRustDeskService(cfg.User.Username); err != nil {
		log.Printf("[ERROR]: could not start RustDesk service, reason: %v", err)
		return err
	}

	return nil
}

func RestartRustDeskService(username string) error {
	if err := StopSystemRustDeskService(); err != nil {
		return err
	}

	if err := StartSystemRustDeskService(); err != nil {
		return err
	}

	if err := StopRustDeskService(username); err != nil {
		return err
	}

	if err := StartRustDeskService(username); err != nil {
		return err
	}

	return nil
}

func StopSystemRustDeskService() error {
	command := "launchctl unload /Library/LaunchDaemons/com.carriez.RustDesk_service.plist"
	cmd := exec.Command("bash", "-c", command)
	return cmd.Run()
}

func StartSystemRustDeskService() error {
	command := "launchctl load /Library/LaunchDaemons/com.carriez.RustDesk_service.plist"
	cmd := exec.Command("bash", "-c", command)
	return cmd.Run()
}

func StopRustDeskService(username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}

	// Reference: https://breardon.home.blog/2019/09/18/sudo-u-vs-launchctl-asuser/
	cmd := exec.Command("/bin/launchctl", "asuser", u.Uid, "launchctl", "unload", "/Library/LaunchAgents/com.carriez.RustDesk_server.plist")
	out, err := cmd.CombinedOutput()
	if err != nil && strings.Contains(string(out), "5") {
		return nil
	}
	return err
}

func StartRustDeskService(username string) error {
	u, err := user.Lookup(username)
	if err != nil {
		return err
	}

	// Reference: https://breardon.home.blog/2019/09/18/sudo-u-vs-launchctl-asuser/
	cmd := exec.Command("/bin/launchctl", "asuser", u.Uid, "launchctl", "load", "/Library/LaunchAgents/com.carriez.RustDesk_server.plist")
	out, err := cmd.CombinedOutput()
	if err != nil && strings.Contains(string(out), "5") {
		return nil
	}
	return err
}
