package rustdesk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nats-io/nats.go"
	openuem_nats "github.com/open-uem/nats"
)

type RustDeskUser struct {
	Username string
	Uid      int
	Gid      int
	Home     string
}

type RustDeskConfig struct {
	User                 *RustDeskUser
	Binary               string
	LaunchArgs           []string
	GetIDArgs            []string
	ConfigFile           string
	IsFlatpak            bool
	UseDirectIPAccess    bool
	UsePermanentPassword bool
	Whitelist            string
}

type RustDeskOptionsEntries struct {
	CustomRendezVousServer string `toml:"custom-rendezvous-server"`
	RelayServer            string `toml:"relay-server"`
	Key                    string `toml:"key"`
	ApiServer              string `toml:"api-server"`
	UseDirectIPAccess      string `toml:"direct-server,omitempty"`
	Whitelist              string `toml:"whitelist,omitempty"`
}

type RustDeskOptions struct {
	Optional RustDeskOptionsEntries `toml:"options"`
}

type RustDeskPassword struct {
	Password string `toml:"password"`
	Salt     string `toml:"salt"`
	EncID    string `toml:"enc_id"`
}

func New() *RustDeskConfig {
	return &RustDeskConfig{}
}

func (cfg *RustDeskConfig) SetRustDeskPassword(config []byte) error {
	// The --password command requires root privileges which is not
	// possible using Flatpak so we've to do a workaround
	// creating an empty RustDesk.toml file with the password
	// encrypted

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

	if !cfg.IsFlatpak {
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
	} else {
		log.Print("[ERROR]: cannot set RustDesk password for flatpak, disable the use of permanent password for this tenant")
		return errors.New("cannot set RustDesk password for flatpak, disable the use of permanent password for this tenant")
	}

	// // Configuration file location
	// configFile := ""
	// rootConfigPath := ""
	// configPath := ""

	// if cfg.IsFlatpak {
	// 	rootConfigPath = filepath.Join(cfg.User.Home, ".var")
	// 	configPath = filepath.Join(rootConfigPath, "app", "com.rustdesk.RustDesk", "config", "rustdesk")
	// 	configFile = filepath.Join(configPath, "RustDesk.toml")
	// } else {
	// 	rootConfigPath = filepath.Join(cfg.User.Home, ".config", "rustdesk")
	// 	configPath = rootConfigPath
	// 	configFile = filepath.Join(configPath, "RustDesk.toml")
	// }

	// currentConfig, err := os.ReadFile(configFile)
	// if err != nil {
	// 	log.Printf("[ERROR]: could not read RustDesk.toml file, reason: %v", err)
	// 	return err
	// }

	// tomlConfig := RustDeskPassword{}
	// if err := toml.Unmarshal(currentConfig, &tomlConfig); err != nil {
	// 	log.Printf("[ERROR]: could not unmarshal RustDesk.toml file, reason: %v", err)
	// 	return err
	// }

	// tomlConfig.Password = rdConfig.PermanentPassword

	// data, err := toml.Marshal(tomlConfig)
	// if err != nil {
	// 	log.Printf("[ERROR]: could not marshal new configuration file, reason: %v", err)
	// 	return err
	// }

	// if err := os.WriteFile(configFile, data, 0600); err != nil {
	// 	log.Printf("[ERROR]: could not write configuration file with new password, reason: %v", err)
	// 	return err
	// }

	return nil
}

// Reference: https://stackoverflow.com/questions/73864379/golang-change-permission-os-chmod-and-os-chowm-recursively
func ChownRecursively(root string, uid int, gid int) error {

	return filepath.Walk(root,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			err = os.Chown(path, uid, gid)
			if err != nil {
				return err
			}
			return nil
		})
}

// Reference: https://leapcell.io/blog/how-to-copy-a-file-in-go
func CopyFile(src, dst string) error {
	// Open the source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destinationFile.Close()

	// Copy the content
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Flush file metadata to disk
	err = destinationFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	return nil
}

func RustDeskRespond(msg *nats.Msg, id string, errMessage string) {
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
