//go:build linux

package rustdesk

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/open-uem/nats"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	"github.com/pelletier/go-toml/v2"
	"github.com/shirou/gopsutil/v3/process"
)

func (cfg *RustDeskConfig) GetInstallationInfo() error {
	rdUser, err := getRustDeskUserInfo()
	if err == nil {
		cfg.User = rdUser
	}

	binPath := "/usr/bin/rustdesk"
	flatpakGlobalPath := "/var/lib/flatpak/exports/bin/com.rustdesk.RustDesk"

	cfg.IsFlatpak = false
	if _, err := os.Stat(binPath); err == nil {
		cfg.Binary = binPath
		cfg.GetIDArgs = []string{"--get-id"}
	} else {
		if _, err := os.Stat(flatpakGlobalPath); err == nil {
			cfg.IsFlatpak = true
			cfg.Binary = "flatpak"
			cfg.LaunchArgs = []string{"run", "com.rustdesk.RustDesk"}
			cfg.GetIDArgs = []string{"run", "com.rustdesk.RustDesk", "--get-id"}
		} else {
			if rdUser != nil {
				flatpakUserPath := filepath.Join(rdUser.Home, "exports", "bin", "com.rustdesk.RustDesk")

				if _, err := os.Stat(flatpakUserPath); err == nil {
					cfg.IsFlatpak = true
					cfg.LaunchArgs = []string{"run", "com.rustdesk.RustDesk"}
					cfg.GetIDArgs = []string{"run", "com.rustdesk.RustDesk", "--get-id"}
				} else {
					return errors.New("RustDesk not found")
				}
			}
		}
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
		log.Println("[INFO]: no RustDesk server settings have been found for tenant, using RustDesk's default settings")
	}

	// Configuration file location
	configFile := ""
	rootConfigPath := ""
	configPath := ""
	if cfg.IsFlatpak {
		if cfg.User == nil || cfg.User.Home == "" {
			log.Println("[ERROR]: Rustdesk was installed with Flatpak, but the agent haven't found which user is logged in, which is required to use this integration")
			return errors.New("Rustdesk was installed with Flatpak, but the agent haven't found which user is logged in, which is required to use this integration")
		}
		rootConfigPath = filepath.Join(cfg.User.Home, ".var")
		configPath = filepath.Join(rootConfigPath, "app", "com.rustdesk.RustDesk", "config", "rustdesk")
		configFile = filepath.Join(configPath, "RustDesk2.toml")
	} else {
		rootConfigPath = filepath.Join("root", ".config", "rustdesk")
		configPath = rootConfigPath
		configFile = filepath.Join(configPath, "RustDesk2.toml")
	}

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

	if cfg.IsFlatpak {
		if err := os.MkdirAll(configPath, 0755); err != nil {
			log.Printf("[ERROR]: could not create directory file for RustDesk configuration, reason: %v", err)
			return err
		}

		if err := ChownRecursively(rootConfigPath, cfg.User.Uid, cfg.User.Gid); err != nil {
			log.Printf("[ERROR]: could not chown directory file for RustDesk configuration, reason: %v", err)
			return err
		}

	}

	if err := os.WriteFile(configFile, rdTOML, 0600); err != nil {
		log.Printf("[ERROR]: could not create TOML file for RustDesk configuration, reason: %v", err)
		return err
	}

	if cfg.IsFlatpak {
		if err := os.Chown(configFile, cfg.User.Uid, cfg.User.Gid); err != nil {
			log.Printf("[ERROR]: could not chown the TOML file for RustDesk configuration, reason: %v", err)
			return err
		}
	}

	// Restart the RustDesk service to apply the new configuration if not flatpak
	if !cfg.IsFlatpak {
		command := "/usr/bin/systemctl restart rustdesk"
		cmd := exec.Command("bash", "-c", command)
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

func (cfg *RustDeskConfig) GetRustDeskID() (string, error) {
	var out []byte
	var err error

	// Get RustDesk ID
	if cfg.User == nil || cfg.User.Username == "" {
		out, err = exec.Command(cfg.Binary, cfg.GetIDArgs...).CombinedOutput()
		if err != nil {
			log.Printf("[ERROR]: could not get RustDesk ID, reason: %v", err)
			return "", err
		}
	} else {
		out, err = runtime.RunAsUserWithOutput(cfg.User.Username, cfg.Binary, cfg.GetIDArgs, true)
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

	if !cfg.IsFlatpak {
		// Restart the RustDesk service
		command := "/usr/bin/systemctl restart rustdesk"
		cmd := exec.Command("bash", "-c", command)
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

func (cfg *RustDeskConfig) ConfigRollBack() error {

	rdUser, err := getRustDeskUserInfo()
	if err != nil {
		return err
	}

	configFile := ""
	if cfg.IsFlatpak {
		configPath := filepath.Join(rdUser.Home, ".var", "app", "com.rustdesk.RustDesk", "config", "rustdesk")
		configFile = filepath.Join(configPath, "RustDesk.toml")
	} else {
		configPath := filepath.Join("root", ".config", "rustdesk")
		configFile = filepath.Join(configPath, "RustDesk.toml")
	}

	// Check if configuration file exists, if exists create a backup
	if _, err := os.Stat(configFile + ".bak"); err == nil {
		if err := os.Rename(configFile+".bak", configFile); err != nil {
			return err
		}
	}

	if cfg.IsFlatpak {
		configPath := filepath.Join(rdUser.Home, ".var", "app", "com.rustdesk.RustDesk", "config", "rustdesk")
		configFile = filepath.Join(configPath, "RustDesk2.toml")
	} else {
		configPath := filepath.Join("root", ".config", "rustdesk")
		configFile = filepath.Join(configPath, "RustDesk2.toml")
	}

	// Check if configuration file exists, if exists create a backup
	if _, err := os.Stat(configFile + ".bak"); err == nil {
		if err := os.Rename(configFile+".bak", configFile); err != nil {
			return err
		}
	}

	// Restart the RustDesk service to apply the new configuration if not flatpak
	if !cfg.IsFlatpak {
		command := "/usr/bin/systemctl restart rustdesk"
		cmd := exec.Command("bash", "-c", command)
		if err := cmd.Run(); err != nil {
			return err
		}
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

	if !cfg.IsFlatpak {
		// Check if RustDesk.toml file exists (where password resides), if exists create a backup unless a previous backup exists to prevent
		// that the admin forgot to revert it (closed the tab)
		configPath := filepath.Join("root", ".config", "rustdesk")
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
	} else {

		rootConfigPath := filepath.Join(cfg.User.Home, ".var")
		configPath := filepath.Join(rootConfigPath, "app", "com.rustdesk.RustDesk", "config", "rustdesk")
		configFile := filepath.Join(configPath, "RustDesk.toml")

		// Check if configuration file exists, if exists read it and create a backup
		if _, err := os.Stat(configFile); err == nil {
			config, err := os.ReadFile(configFile)
			if err != nil {
				log.Printf("[ERROR]: could not read RustDesk.toml config file reason: %v", err)
				return err
			}

			backupPath := configFile + ".bak"
			if _, err := os.Stat(backupPath); err != nil {
				if err := os.Rename(configFile, backupPath); err != nil {
					return err
				}
			}

			// Read TOML
			cfgTOML := RustDeskPassword{}
			toml.Unmarshal(config, &cfgTOML)

			cfgTOML.Password = rdConfig.PermanentPassword

			// Write new configuration
			rdTOML, err := toml.Marshal(cfgTOML)
			if err != nil {
				log.Printf("[ERROR]: could not marshall TOML file for RustDesk configuration, reason: %v", err)
				return err
			}

			if err := os.WriteFile(configFile, rdTOML, 0600); err != nil {
				log.Printf("[ERROR]: could not create TOML file for RustDesk configuration, reason: %v", err)
				return err
			}
		} else {
			//
			log.Print("[ERROR]: cannot set RustDesk password for flatpak, disable the use of permanent password for this tenant")
			return errors.New("cannot set RustDesk password for flatpak, disable the use of permanent password for this tenant")
		}
	}

	return nil
}
