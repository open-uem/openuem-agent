//go:build windows

package remotedesktop

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	openuem_utils "github.com/open-uem/utils"
	"golang.org/x/sys/windows/registry"
	"gopkg.in/ini.v1"
)

func (rd *RemoteDesktopService) Start(pin string, notifyUser bool) {
	cwd, err := openuem_utils.GetWd()
	if err != nil {
		log.Printf("[ERROR]: could not get working directory, reason: %v\n", err)
		return
	}

	// Show PIN to user if needed
	if notifyUser {
		go func() {
			if err := runtime.RunAsUser(filepath.Join(cwd, "openuem-messenger.exe"), []string{"info", "--message", pin, "--type", "pin"}); err != nil {
				log.Printf("[ERROR]: could not show PIN message to user, reason: %v\n", err)
			}
		}()
	}

	// Configure Remote Desktop service
	if err := rd.Configure(); err != nil {
		log.Printf("[ERROR]: could not configure Remote Desktop service, reason: %v\n", err)
		return
	}

	// Save PIN
	if err := rd.SavePIN(pin); err != nil {
		log.Printf("[ERROR]: could not save PIN before Remote Desktop service is started, reason: %v\n", err)
		return
	}

	// Start Remote Desktop service
	go func() {
		vncPort := ""
		if err := rd.StartService(vncPort); err != nil {
			log.Printf("[ERROR]: could not start Remote Desktop service, reason: %v", err)
			return
		}
		log.Println("[INFO]: the remote desktop service should have been started")
	}()

	// Start VNC Proxy
	if rd.RequiresVNCProxy {
		port := ""
		go rd.StartVNCProxy(port)
	}
}

func (rd *RemoteDesktopService) Stop() {
	if rd.RequiresVNCProxy {
		// Stop proxy
		if err := rd.Proxy.Close(); err != nil {
			log.Printf("[ERROR]: could not stop VNC proxy, reason: %v\n", err)
		}
	}

	// Create new random PIN
	pin, err := openuem_utils.GenerateRandomPIN()
	if err != nil {
		log.Printf("[ERROR]: could not generate random PIN, reason: %v\n", err)
		return
	}

	// Save PIN
	if err := rd.SavePIN(pin); err != nil {
		log.Printf("[ERROR]: could not save PIN before Remote Desktop service is started, reason: %v\n", err)
		return
	}

	// Stop gracefully Remote Desktop service
	if err := rd.StopService(); err != nil {
		log.Printf("[ERROR]: could not stop the remote desktop service, reason: %v", err)
	}
	log.Println("[INFO]: the remote desktop service has been stopped")

}

func GetSupportedRemoteDesktopService(agentOS, sid, proxyPort string) (*RemoteDesktopService, error) {
	supportedServers := map[string]RemoteDesktopService{
		"TightVNC": {
			RequiresVNCProxy: true,
			StartService: func(vncPort string) error {
				return runtime.RunAsUser(`C:\Program Files\TightVNC\tvnserver.exe`, nil)
			},
			StopService: func() error {
				args := []string{"-controlapp", "-shutdown"}
				if err := runtime.RunAsUser(`C:\Program Files\TightVNC\tvnserver.exe`, args); err != nil {
					return err
				}

				// Kill Remote Desktop service as some remnants can be there
				if err := runtime.RunAsUser("taskkill", []string{"/F", "/T", "/IM", "tvnserver.exe"}); err != nil {
					log.Printf("Remote Desktop service kill error, %v\n", err)
				}
				return nil
			},
			Configure: func() error {
				k, err := registry.OpenKey(registry.USERS, sid+`\SOFTWARE\TightVNC\Server`, registry.QUERY_VALUE)
				if err == registry.ErrNotExist {
					k, err = registry.OpenKey(registry.USERS, sid+`\SOFTWARE`, registry.SET_VALUE)
					if err != nil {
						return err
					}
					k, _, err = registry.CreateKey(k, "TightVNC", registry.CREATE_SUB_KEY)
					if err != nil {
						return err
					}

					k, _, err = registry.CreateKey(k, "Server", registry.CREATE_SUB_KEY)
					if err != nil {
						return err
					}
				}

				k, err = registry.OpenKey(registry.USERS, sid+`\SOFTWARE\TightVNC\Server`, registry.SET_VALUE)
				if err != nil {
					return err
				}

				err = k.SetDWordValue("AllowLoopback", 1)
				if err != nil {
					return err
				}

				err = k.SetDWordValue("RemoveWallpaper", 0)
				if err != nil {
					return err
				}

				return nil
			},
			SavePIN: func(pin string) error {
				encryptedPIN := DESEncode(pin)
				k, err := registry.OpenKey(registry.USERS, sid+`\SOFTWARE\TightVNC\Server`, registry.SET_VALUE)
				if err != nil {
					return err
				}

				err = k.SetBinaryValue("Password", encryptedPIN)
				if err != nil {
					return err
				}

				log.Println("[INFO]: PIN saved to registry")
				return nil
			},
		},
		"UltraVNC": {
			RequiresVNCProxy: true,
			StartService: func(vncPort string) error {
				return runtime.RunAsUser(`C:\Program Files\uvnc bvba\UltraVNC\winvnc.exe`, nil)
			},
			StopService: func() error {
				args := []string{"-kill"}
				return runtime.RunAsUser(`C:\Program Files\uvnc bvba\UltraVNC\winvnc.exe`, args)
			},
			Configure: func() error {
				iniFile := `C:\Program Files\uvnc bvba\UltraVNC\ultravnc.ini`
				cfg, err := ini.Load(iniFile)
				if err != nil {
					log.Println(`C:\Program Files\uvnc bvba\UltraVNC\ultravnc.ini cannot be opened`)
					return err
				}

				adminSection := cfg.Section("admin")
				adminSection.Key("LoopbackOnly").SetValue("1")
				adminSection.Key("FileTransferEnabled").SetValue("0")
				adminSection.Key("FTUserImpersonation").SetValue("0")
				adminSection.Key("HTTPConnect").SetValue("0")

				if err := cfg.SaveTo(iniFile); err != nil {
					log.Printf("[ERROR]: could not save UltraVNC ini file, reason: %v\n", err)
					return err
				}
				log.Println("[INFO]: Remote Desktop service configured")
				return nil
			},
			SavePIN: func(pin string) error {
				iniFile := `C:\Program Files\uvnc bvba\UltraVNC\ultravnc.ini`
				encryptedPIN := UltraVNCEncrypt(pin)

				cfg, err := ini.Load(iniFile)
				if err != nil {
					return nil
				}

				cfg.Section("ultravnc").Key("passwd").SetValue(encryptedPIN)
				if err := cfg.SaveTo(iniFile); err != nil {
					log.Printf("[ERROR]: could not save file, reason: %v\n", err)
				}
				log.Println("[INFO]: PIN saved to file")
				return nil
			},
		},
		"TigerVNC": {
			RequiresVNCProxy: true,
			StartService: func(vncPort string) error {
				return runtime.RunAsUser(`C:\Program Files\TigerVNC Server\winvnc4.exe`, nil)
			},
			StopService: func() error {
				args := []string{"/F", "/T", "/IM", "winvnc4.exe"}
				return runtime.RunAsUser("taskkill", args)
			},
			Configure: func() error {
				_, err := registry.OpenKey(registry.USERS, sid+`\SOFTWARE\TigerVNC`, registry.QUERY_VALUE)
				if err == registry.ErrNotExist {
					k, err := registry.OpenKey(registry.USERS, sid+`\SOFTWARE`, registry.QUERY_VALUE)
					if err != nil {
						return err
					}
					k, _, err = registry.CreateKey(k, "TigerVNC", registry.CREATE_SUB_KEY)
					if err != nil {
						return err
					}

					k, _, err = registry.CreateKey(k, "WinVNC4", registry.CREATE_SUB_KEY)
					if err != nil {
						return err
					}

					err = k.SetDWordValue("LocalHost", 1)
					if err != nil {
						return err
					}
				}

				return nil
			},
			SavePIN: func(pin string) error {
				encryptedPIN := DESEncode(pin)
				k, err := registry.OpenKey(registry.USERS, sid+`\SOFTWARE\TigerVNC\WinVNC4`, registry.SET_VALUE)
				if err != nil {
					return err
				}

				err = k.SetBinaryValue("Password", encryptedPIN)
				if err != nil {
					return err
				}
				log.Println("[INFO]: PIN saved to registry")
				return nil
			},
		},
	}

	supported := GetSupportedRemoteDesktop(agentOS)
	if supported == "" {
		return nil, fmt.Errorf("no supported Remote Desktop service")
	}

	server := supportedServers[supported]
	server.Name = supported
	return &server, nil
}

func GetSupportedRemoteDesktop(agentOS string) string {
	if agentOS == "windows" {
		if _, err := os.Stat(`C:\Program Files\TightVNC\tvnserver.exe`); err == nil {
			return "TightVNC"
		}
		if _, err := os.Stat(`C:\Program Files\uvnc bvba\UltraVNC\winvnc.exe`); err == nil {
			return "UltraVNC"
		}
		if _, err := os.Stat(`C:\Program Files\TigerVNC Server\winvnc4.exe`); err == nil {
			return "TigerVNC"
		}
	}

	return ""
}

func GetAgentOS() string {
	return "windows"
}
