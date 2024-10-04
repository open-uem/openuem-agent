package vnc

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/evangwt/go-vncproxy"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
	"golang.org/x/sys/windows/registry"
	"gopkg.in/ini.v1"
)

type VNCServer struct {
	Name            string
	StartCommand    string
	StopCommand     string
	StopCommandArgs []string
	Configure       func() error
	ConfigureAsUser bool
	SavePIN         func(pin string) error
	Proxy           *echo.Echo
	ProxyCert       string
	ProxyKey        string
}

func New(certPath, keyPath, sid string) (*VNCServer, error) {
	server, err := GetSupportedVNCServer(sid)
	if err != nil {
		return nil, err
	}
	server.Proxy = echo.New()
	server.ProxyCert = certPath
	server.ProxyKey = keyPath
	return server, nil
}

func (vnc *VNCServer) Start() {

	// Create PIN
	pin, err := GenerateRandomPIN()
	if err != nil {
		log.Printf("[ERROR]: could not generate random PIN, reason: %v\n", err)
		return
	}

	// Show PIN to user
	go func() {
		message := fmt.Sprintf("This is the PIN for your remote assistance: %s", pin)
		if err := RunAsUser(`C:\Program Files\OpenUEM Agent\openuem_message.exe`, []string{"info", "--message", message, "--title", "OpenUEM - Remote Assistance"}); err != nil {
			log.Printf("[ERROR]: could not show test message to user, reason: %v\n", err)
		}
	}()

	// Configure VNC server
	if err := vnc.Configure(); err != nil {
		log.Printf("[ERROR]: could not configure VNC server, reason: %v\n", err)
		return
	}

	// Save PIN
	if err := vnc.SavePIN(pin); err != nil {
		log.Printf("[ERROR]: could not save PIN before VNC is started, reason: %v\n", err)
		return
	}

	// Start VNC server
	go RunAsUser(vnc.StartCommand, nil)

	// Start VNC Proxy
	go vnc.StartProxy()
}

func (vnc *VNCServer) Stop() {
	// Stop proxy
	if err := vnc.Proxy.Close(); err != nil {
		log.Printf("[ERROR]: could not stop VNC proxy, reason: %v\n", err)
	}

	// Create new random PIN
	pin, err := GenerateRandomPIN()
	if err != nil {
		log.Printf("[ERROR]: could not generate random PIN, reason: %v\n", err)
		return
	}

	// Save PIN
	if err := vnc.SavePIN(pin); err != nil {
		log.Printf("[ERROR]: could not save PIN before VNC is started, reason: %v\n", err)
		return
	}

	// Stop VNC server
	go RunAsUser(vnc.StopCommand, vnc.StopCommandArgs)
}

func (vnc *VNCServer) StartProxy() {
	// Launch proxy only if port is available
	_, err := net.DialTimeout("tcp", ":1443", 5*time.Second)
	if err != nil {
		vncProxy := vncproxy.New(&vncproxy.Config{
			LogLevel: vncproxy.InfoFlag,
			TokenHandler: func(r *http.Request) (addr string, err error) {
				return ":5900", nil
			},
		})
		vnc.Proxy.GET("/ws", func(ctx echo.Context) error {
			h := websocket.Handler(vncProxy.ServeWS)
			h.ServeHTTP(ctx.Response().Writer, ctx.Request())
			return nil
		})
		fmt.Println("[INFO]: NoVNC proxy server started")

		if err := vnc.Proxy.StartTLS(":1443", vnc.ProxyCert, vnc.ProxyKey); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}

}

func GetSupportedVNCServer(sid string) (*VNCServer, error) {
	supportedServers := map[string]VNCServer{
		"TightVNC": {
			StartCommand:    `C:\Program Files\TightVNC\tvnserver.exe`,
			StopCommand:     `C:\Program Files\TightVNC\tvnserver.exe`,
			StopCommandArgs: []string{"-controlapp", "-shutdown"},
			ConfigureAsUser: true,
			Configure: func() error {
				k, err := registry.OpenKey(registry.USERS, sid+`\SOFTWARE\TightVNC\Server`, registry.SET_VALUE)
				if err != nil {
					return err
				}

				// Allow loopback connections for NoVNC proxy
				err = k.SetDWordValue("AllowLoopback", 1)
				if err != nil {
					return err
				}

				fmt.Println("[INFO]: VNC configured")
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
				fmt.Println("[INFO]: PIN saved to registry")
				return nil
			},
		},
		"UltraVNC": {
			StartCommand:    `C:\Program Files\uvnc bvba\UltraVNC\winvnc.exe`,
			StopCommand:     `C:\Program Files\uvnc bvba\UltraVNC\winvnc.exe`,
			StopCommandArgs: []string{"-kill"},
			ConfigureAsUser: false,
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
				fmt.Println("[INFO]: VNC configured")
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
				fmt.Println("[INFO]: PIN saved to file")
				return nil
			},
		},
		"TigerVNC": {
			StartCommand:    `C:\Program Files\TigerVNC Server\winvnc4.exe`,
			StopCommand:     "taskkill",
			StopCommandArgs: []string{"/F", "/T", "/IM", "winvnc4.exe"},
			ConfigureAsUser: true,
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
				} else {
					return err
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
				fmt.Println("[INFO]: PIN saved to registry")
				return nil
			},
		},
	}

	for name, server := range supportedServers {
		if _, err := os.Stat(server.StartCommand); err == nil {
			server.Name = name
			return &server, nil
		}
	}
	return nil, fmt.Errorf("no supported VNC server")
}
