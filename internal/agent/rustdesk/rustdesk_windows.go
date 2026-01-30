//go:build windows

package rustdesk

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/open-uem/nats"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/report"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	openuem_utils "github.com/open-uem/utils"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/sys/windows/svc"
)

func (cfg *RustDeskConfig) GetInstallationInfo() error {
	binPath := "C:\\Program Files\\RustDesk\\rustdesk.exe"

	if _, err := os.Stat(binPath); err == nil {
		cfg.Binary = binPath
		cfg.GetIDArgs = []string{"--get-id"}
	} else {
		return errors.New("RustDesk not found")
	}

	return nil
}

func (cfg *RustDeskConfig) Configure(config []byte) error {

	// Unmarshal configuration data
	var rdConfig nats.RustDesk
	if err := json.Unmarshal(config, &rdConfig); err != nil {
		log.Println("[ERROR]: could not unmarshall RustDesk configuration")
		return err
	}

	if rdConfig.CustomRendezVousServer == "" &&
		rdConfig.RelayServer == "" &&
		rdConfig.Key == "" &&
		rdConfig.APIServer == "" &&
		!rdConfig.DirectIPAccess {
		log.Println("[INFO]: no RustDesk settings has been found for tenant, using RustDesk's default settings")
	}

	// Configuration file location
	configFile := ""
	configPath := ""

	configPath = "C:\\Windows\\ServiceProfiles\\LocalService\\AppData\\Roaming\\RustDesk\\config"

	// Create TOML file
	cfgTOML := RustDeskOptions{
		Optional: RustDeskOptionsEntries{
			CustomRendezVousServer:  rdConfig.CustomRendezVousServer,
			RelayServer:             rdConfig.RelayServer,
			Key:                     rdConfig.Key,
			ApiServer:               rdConfig.APIServer,
			TemporaryPasswordLength: strconv.Itoa(rdConfig.TemporaryPasswordLength),
			VerificationMethod:      rdConfig.VerificationMethod,
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

	// Check if RustDesk.toml file exists, if exists create a backup unless a previous backup exists to prevent
	// that the admin forgot to revert it (closed the tab)
	configFile = filepath.Join(configPath, "RustDesk.toml")
	if _, err := os.Stat(configFile); err == nil {
		backupPath := configFile + ".bak"
		if _, err := os.Stat(backupPath); err != nil {
			if err := CopyFile(configFile, backupPath); err != nil {
				return err
			}
		}
	}

	// Check if RustDesk2.toml file exists, if exists create a backup unless a previous backup exists to prevent
	// that the admin forgot to revert it (closed the tab)
	configFile = filepath.Join(configPath, "RustDesk2.toml")
	if _, err := os.Stat(configFile); err == nil {
		backupPath := configFile + ".bak"
		if _, err := os.Stat(backupPath); err != nil {
			if err := CopyFile(configFile, backupPath); err != nil {
				return err
			}
		}
	}

	if err := os.MkdirAll(configPath, 0755); err != nil {
		log.Printf("[ERROR]: could not create directory file for RustDesk configuration, reason: %v", err)
		return err
	}

	if err := os.WriteFile(configFile, rdTOML, 0600); err != nil {
		log.Printf("[ERROR]: could not create TOML file for RustDesk configuration, reason: %v", err)
		return err
	}

	// Restart RustDesk service after configuration changes
	if err := openuem_utils.WindowsSvcControl("RustDesk", svc.Stop, svc.Stopped); err != nil {
		log.Printf("[ERROR]: could not stop RustDesk service, reason: %v\n", err)
		return err
	}

	// Start service
	if err := openuem_utils.WindowsStartService("RustDesk"); err != nil {
		log.Printf("[ERROR]: could not start RustDesk service, reason: %v\n", err)
		return err
	}

	return nil
}

func (cfg *RustDeskConfig) GetRustDeskID() (string, error) {
	var out []byte
	var err error

	// Get RustDesk ID
	username, err := report.GetLoggedOnUsername()
	if err == nil || username == "" {
		out, err = exec.Command(cfg.Binary, cfg.GetIDArgs...).CombinedOutput()
		if err != nil {
			log.Printf("[ERROR]: could not get RustDesk ID, reason: %v", err)
			return "", err
		}
	} else {
		out, err = runtime.RunAsUserWithOutput(cfg.Binary, cfg.GetIDArgs)
		if err != nil {
			log.Printf("[ERROR]: could not get RustDesk ID, reason: %v", err)
			return "", err
		}
	}

	id := strings.TrimSpace(string(out))
	_, err = strconv.Atoi(id)
	if err != nil {
		log.Printf("[ERROR]: RustDesk ID is not a number, reason: %v", err)
		return "", err
	}

	return id, nil
}

func (cfg *RustDeskConfig) KillRustDeskProcess() error {
	var err error

	args := []string{"/F", "/T", "/IM", "rustdesk.exe"}

	// Get RustDesk ID
	username, err := report.GetLoggedOnUsername()
	if err == nil || username == "" {
		out, err := exec.Command(cfg.Binary, cfg.GetIDArgs...).CombinedOutput()
		if err != nil {
			if !strings.Contains(err.Error(), "128") && !strings.Contains(err.Error(), "255") {
				log.Printf("[WARN]: could not kill RustDesk app, reason: %s, %v", string(out), err)
				return fmt.Errorf("[WARN]: could not kill RustDesk app, reason: %s, %v", string(out), err)
			}
		}
	} else {
		if err := runtime.RunAsUser("taskkill", args); err != nil {
			if !strings.Contains(err.Error(), "128") && !strings.Contains(err.Error(), "255") {
				log.Printf("[WARN]: could not kill RustDesk app, reason: %v", err)
				return fmt.Errorf("[WARN]: could not kill RustDesk app, reason: %v", err)
			}
		}
	}
	return nil
}

func (cfg *RustDeskConfig) ConfigRollBack() error {
	configFile := "C:\\Windows\\ServiceProfiles\\LocalService\\AppData\\Roaming\\RustDesk\\config\\RustDesk.toml"
	// Check if configuration file backup exists, if exists revert the backup
	if _, err := os.Stat(configFile + ".bak"); err == nil {
		if err := os.Rename(configFile+".bak", configFile); err != nil {
			return err
		}
	}

	configFile2 := "C:\\Windows\\ServiceProfiles\\LocalService\\AppData\\Roaming\\RustDesk\\config\\RustDesk2.toml"
	// Check if configuration file backup exists, if exists revert the backup
	if _, err := os.Stat(configFile2 + ".bak"); err == nil {
		if err := os.Rename(configFile2+".bak", configFile2); err != nil {
			return err
		}
	}

	// Restart RustDesk service after configuration changes
	if err := openuem_utils.WindowsSvcControl("RustDesk", svc.Stop, svc.Stopped); err != nil {
		log.Printf("[ERROR]: could not stop RustDesk service, reason: %v\n", err)
		return err
	}

	// Start service
	if err := openuem_utils.WindowsStartService("RustDesk"); err != nil {
		log.Printf("[ERROR]: could not start RustDesk service, reason: %v\n", err)
		return err
	}

	return nil
}

func (cfg *RustDeskConfig) SetRustDeskPassword(config []byte) error {
	// The --password command requires root privileges which is not
	// possible using Flatpak so we've to do a workaround
	// adding the the password in clear to RustDesk.toml
	// this password is encrypted as soon as the RustDesk app is

	// Unmarshal configuration data
	var rdConfig openuem_nats.RustDesk
	if err := json.Unmarshal(config, &rdConfig); err != nil {
		log.Println("[ERROR]: could not unmarshall RustDesk configuration")
		return err
	}

	// If no password is set skip
	if rdConfig.PermanentPassword == "" {
		return nil
	}

	// Check if RustDesk.toml file exists (where password resides), if exists create a backup unless a previous backup exists to prevent
	// that the admin forgot to revert it (closed the tab)
	configPath := "C:\\Windows\\ServiceProfiles\\LocalService\\AppData\\Roaming\\RustDesk\\config"
	configFile := filepath.Join(configPath, "RustDesk.toml")
	if _, err := os.Stat(configFile); err == nil {
		backupPath := configFile + ".bak"
		if _, err := os.Stat(backupPath); err != nil {
			if err := CopyFile(configFile, backupPath); err != nil {
				return err
			}
		}
	}

	// Set RustDesk password using command
	cmd := exec.Command(cfg.Binary, "--password", rdConfig.PermanentPassword)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[ERROR]: could not execute RustDesk command to set password, reason: %v", err)
		return err
	}

	if strings.TrimSpace(string(out)) != "Done!" {
		log.Printf("[ERROR]: could not change RustDesk password, reason: %s", string(out))
		return err
	}

	return nil
}
