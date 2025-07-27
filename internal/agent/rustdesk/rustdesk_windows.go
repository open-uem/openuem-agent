//go:build windows

package rustdesk

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	"github.com/pelletier/go-toml/v2"
)

type RustDeskConfig struct {
	User       *RustDeskUser
	Binary     string
	LaunchArgs []string
	GetIDArgs  []string
	ConfigFile string
}

type RustDeskUser struct {
	Username string
	Uid      int
	Gid      int
	Home     string
}

func New() *RustDeskConfig {
	return &RustDeskConfig{}
}

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
		return nil
	}

	// Configuration file location
	configFile := ""
	configPath := ""

	configPath = "C:\\Windows\\ServiceProfiles\\LocalService\\AppData\\Roaming\\RustDesk\\config"
	configFile = filepath.Join(configPath, "RustDesk2.toml")

	// Create TOML file
	cfgTOML := RustDeskOptions{
		Optional: RustDeskOptionsEntries{
			CustomRendezVousServer: rdConfig.CustomRendezVousServer,
			RelayServer:            rdConfig.RelayServer,
			Key:                    rdConfig.Key,
			ApiServer:              rdConfig.APIServer,
		},
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
	}

	if err := os.MkdirAll(configPath, 0755); err != nil {
		log.Printf("[ERROR]: could not create directory file for RustDesk configuration, reason: %v", err)
		return err
	}

	if err := os.WriteFile(configFile, rdTOML, 0600); err != nil {
		log.Printf("[ERROR]: could not create TOML file for RustDesk configuration, reason: %v", err)
		return err
	}

	return nil
}

func (cfg *RustDeskConfig) LaunchRustDesk() error {
	return runtime.RunAsUser(cfg.Binary, cfg.LaunchArgs)
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
