//go:build windows

package rustdesk

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/open-uem/nats"
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
		rdConfig.APIServer == "" {
		log.Println("[INFO]: no RustDesk settings has been found for tenant, using RustDesk's default settings")
	}

	// Configuration file location
	configFile := ""
	configPath := ""

	configPath = "C:\\Windows\\ServiceProfiles\\LocalService\\AppData\\Roaming\\RustDesk\\config"
	configFile = filepath.Join(configPath, "RustDesk2.toml")

	// Create TOML file
	cfgTOML := RustDeskOptions{
		Optional: RustDeskOptionsEntries{},
	}

	if rdConfig.DirectIPAccess {
		cfgTOML.Optional.UseDirectIPAccess = "Y"
	} else {
		cfgTOML.Optional.CustomRendezVousServer = rdConfig.CustomRendezVousServer
		cfgTOML.Optional.RelayServer = rdConfig.RelayServer
		cfgTOML.Optional.Key = rdConfig.Key
		cfgTOML.Optional.ApiServer = rdConfig.APIServer
	}

	if rdConfig.Whitelist != "" {
		cfgTOML.Optional.Whitelist = rdConfig.Whitelist
	}

	rdTOML, err := toml.Marshal(cfgTOML)
	if err != nil {
		log.Printf("[ERROR]: could not marshall TOML file for RustDesk configuration, reason: %v", err)
		return err
	}

	// Check if configuration file exists, if exists create a backup unless a previous backup exists to prevent
	// that the admin forgot to revert it (closed the tab)
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

func (cfg *RustDeskConfig) LaunchRustDesk() error {
	return runtime.RunAsUserInBackground(cfg.Binary, cfg.LaunchArgs)
}

func (cfg *RustDeskConfig) GetRustDeskID() (string, error) {
	// Get RustDesk ID
	out, err := runtime.RunAsUserWithOutput(cfg.Binary, cfg.GetIDArgs)
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

func (cfg *RustDeskConfig) KillRustDeskProcess() error {
	args := []string{"/F", "/T", "/IM", "rustdesk.exe"}
	if err := runtime.RunAsUser("taskkill", args); err != nil {
		if !strings.Contains(err.Error(), "128") && !strings.Contains(err.Error(), "255") {
			log.Printf("[WARN]: could not kill RustDesk app, reason: %v", err)
			return fmt.Errorf("[WARN]: could not kill RustDesk app, reason: %v", err)
		}
	}
	return nil
}

func (cfg *RustDeskConfig) ConfigRollBack() error {
	configFile := "C:\\Windows\\ServiceProfiles\\LocalService\\AppData\\Roaming\\RustDesk\\config\\RustDesk2.toml"

	// Check if configuration file exists, if exists create a backup
	if _, err := os.Stat(configFile + ".bak"); err == nil {
		if err := os.Rename(configFile+".bak", configFile); err != nil {
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
