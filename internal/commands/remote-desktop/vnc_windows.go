//go:build windows

package remotedesktop

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/evangwt/go-vncproxy"
	"github.com/labstack/echo/v4"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	openuem_utils "github.com/open-uem/utils"
	"golang.org/x/net/websocket"
	"golang.org/x/sys/windows/registry"
	"gopkg.in/ini.v1"
)

func (rd *RemoteDesktopService) StartVNCProxy() {
	log.Printf("[INFO]: starting VNC proxy on port %s\n", rd.ProxyPort)
	// Launch proxy only if port is available
	_, err := net.DialTimeout("tcp", ":"+rd.ProxyPort, 5*time.Second)
	if err != nil {
		vncProxy := vncproxy.New(&vncproxy.Config{
			LogLevel: vncproxy.InfoFlag,
			TokenHandler: func(r *http.Request) (addr string, err error) {
				return ":5900", nil
			},
		})
		rd.Proxy.GET("/ws", func(ctx echo.Context) error {
			h := websocket.Handler(vncProxy.ServeWS)
			h.ServeHTTP(ctx.Response().Writer, ctx.Request())
			return nil
		})

		log.Println("[INFO]: NoVNC proxy server started")

		if err := rd.Proxy.StartTLS(":"+rd.ProxyPort, rd.ProxyCert, rd.ProxyKey); err != http.ErrServerClosed {
			log.Printf("[ERROR]: could not start VNC proxy\n, %v", err)
		}

	} else {
		log.Printf("[ERROR]: VNC proxy port %s is not available\n", rd.ProxyPort)
	}
}

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
	if _, err := rd.Configure(); err != nil {
		log.Printf("[ERROR]: could not configure Remote Desktop service, reason: %v\n", err)
		return
	}

	// Save PIN
	if err := rd.SavePIN(pin); err != nil {
		log.Printf("[ERROR]: could not save PIN before Remote Desktop service is started, reason: %v\n", err)
		return
	}

	// Start Remote Desktop service
	go runtime.RunAsUser(rd.StartCommand, nil)

	// Start VNC Proxy
	if rd.RequiresVNCProxy {
		go rd.StartVNCProxy()
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
	if rd.StopCommand != "" {
		err := runtime.RunAsUser(rd.StopCommand, rd.StopCommandArgs)
		if err != nil {
			log.Printf("Remote Desktop service stop error, %v\n", err)
		}
	}

	// Kill Remote Desktop service as some remains can be there
	if rd.KillCommand != "" {
		err := runtime.RunAsUser(rd.KillCommand, rd.KillCommandArgs)
		if err != nil {
			log.Printf("Remote Desktop service kill error, %v\n", err)
		}
	}
}

func GetSupportedRemoteDesktopService(agentOS, sid, proxyPort string) (*RemoteDesktopService, error) {
	supportedServers := map[string]RemoteDesktopService{
		"TightVNC": {
			RequiresVNCProxy: true,
			StartCommand:     `C:\Program Files\TightVNC\tvnserver.exe`,
			StopCommand:      `C:\Program Files\TightVNC\tvnserver.exe`,
			StopCommandArgs:  []string{"-controlapp", "-shutdown"},
			KillCommand:      "taskkill",
			KillCommandArgs:  []string{"/F", "/T", "/IM", "tvnserver.exe"},
			ConfigureAsUser:  true,
			Configure: func() (string, error) {
				k, err := registry.OpenKey(registry.USERS, sid+`\SOFTWARE\TightVNC\Server`, registry.QUERY_VALUE)
				if err == registry.ErrNotExist {
					k, err = registry.OpenKey(registry.USERS, sid+`\SOFTWARE`, registry.SET_VALUE)
					if err != nil {
						return "", err
					}
					k, _, err = registry.CreateKey(k, "TightVNC", registry.CREATE_SUB_KEY)
					if err != nil {
						return "", err
					}

					k, _, err = registry.CreateKey(k, "Server", registry.CREATE_SUB_KEY)
					if err != nil {
						return "", err
					}
				}

				k, err = registry.OpenKey(registry.USERS, sid+`\SOFTWARE\TightVNC\Server`, registry.SET_VALUE)
				if err != nil {
					return "", err
				}

				err = k.SetDWordValue("AllowLoopback", 1)
				if err != nil {
					return "", err
				}

				err = k.SetDWordValue("RemoveWallpaper", 0)
				if err != nil {
					return "", err
				}

				return "", nil
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
			StartCommand:     `C:\Program Files\uvnc bvba\UltraVNC\winvnc.exe`,
			StopCommand:      `C:\Program Files\uvnc bvba\UltraVNC\winvnc.exe`,
			StopCommandArgs:  []string{"-kill"},
			ConfigureAsUser:  false,
			Configure: func() (string, error) {
				iniFile := `C:\Program Files\uvnc bvba\UltraVNC\ultravnc.ini`
				cfg, err := ini.Load(iniFile)
				if err != nil {
					log.Println(`C:\Program Files\uvnc bvba\UltraVNC\ultravnc.ini cannot be opened`)
					return "", err
				}

				adminSection := cfg.Section("admin")
				adminSection.Key("LoopbackOnly").SetValue("1")
				adminSection.Key("FileTransferEnabled").SetValue("0")
				adminSection.Key("FTUserImpersonation").SetValue("0")
				adminSection.Key("HTTPConnect").SetValue("0")

				if err := cfg.SaveTo(iniFile); err != nil {
					log.Printf("[ERROR]: could not save UltraVNC ini file, reason: %v\n", err)
					return "", err
				}
				log.Println("[INFO]: Remote Desktop service configured")
				return "", nil
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
			StartCommand:     `C:\Program Files\TigerVNC Server\winvnc4.exe`,
			StopCommand:      "taskkill",
			StopCommandArgs:  []string{"/F", "/T", "/IM", "winvnc4.exe"},
			ConfigureAsUser:  true,
			Configure: func() (string, error) {

				_, err := registry.OpenKey(registry.USERS, sid+`\SOFTWARE\TigerVNC`, registry.QUERY_VALUE)
				if err == registry.ErrNotExist {
					k, err := registry.OpenKey(registry.USERS, sid+`\SOFTWARE`, registry.QUERY_VALUE)
					if err != nil {
						return "", err
					}
					k, _, err = registry.CreateKey(k, "TigerVNC", registry.CREATE_SUB_KEY)
					if err != nil {
						return "", err
					}

					k, _, err = registry.CreateKey(k, "WinVNC4", registry.CREATE_SUB_KEY)
					if err != nil {
						return "", err
					}

					err = k.SetDWordValue("LocalHost", 1)
					if err != nil {
						return "", err
					}
				}

				return "", nil
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
