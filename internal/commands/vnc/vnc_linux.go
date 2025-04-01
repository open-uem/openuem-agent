//go:build linux

package vnc

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/evangwt/go-vncproxy"
	"github.com/labstack/echo/v4"
	openuem_utils "github.com/open-uem/utils"
	"golang.org/x/net/websocket"
)

func (vnc *VNCServer) Start(pin string, notifyUser bool) {
	cwd, err := openuem_utils.GetWd()
	if err != nil {
		log.Printf("[ERROR]: could not get working directory, reason: %v\n", err)
		return
	}

	// Show PIN to user if needed
	if notifyUser {
		go func() {
			if err := RunAsUser(filepath.Join(cwd, "openuem-messenger"), []string{"info", "--message", pin, "--type", "pin"}, true); err != nil {
				log.Printf("[ERROR]: could not show PIN message to user, reason: %v\n", err)
			}
		}()
	}

	// Configure VNC server
	port, err := vnc.Configure()
	if err != nil {
		log.Printf("[ERROR]: could not configure VNC server, reason: %v\n", err)
		return
	}

	// Save PIN
	if err := vnc.SavePIN(pin); err != nil {
		log.Printf("[ERROR]: could not save PIN before VNC is started, reason: %v\n", err)
		return
	}

	// Start VNC server
	go func() {
		// Get logged in username
		username, err := GetLoggedInUser()
		if err != nil {
			log.Printf("[ERROR]: could not get logged in Username, reason: %v\n", err)
		}

		xauthorityEnv, err := getXAuthority()
		if err != nil {
			log.Printf("[ERROR]: could not get XAUTHORITY env, reason: %v\n", err)
		}

		xauthority := strings.TrimPrefix(xauthorityEnv, "XAUTHORITY=")

		if vnc.SystemctlCommand == "" {
			if err := RunAsUser(vnc.StartCommand, vnc.StartCommandArgsFunc(username, port, xauthority), true); err != nil {
				log.Printf("[ERROR]: could not start VNC server, reason: %v", err)
			}
		} else {
			command := fmt.Sprintf("%s %s", vnc.SystemctlCommand, strings.Join(vnc.StartCommandArgsFunc(username, port, xauthority), " "))
			cmd := exec.Command("bash", "-c", command)
			if err := cmd.Run(); err != nil {
				log.Println("command to execute: ", vnc.SystemctlCommand, strings.Join(vnc.StartCommandArgsFunc(username, port, xauthority), " "))
				log.Printf("[ERROR]: could not start VNC server using %s, reason: %v", command, err)
			}
		}
	}()

	// Start VNC Proxy
	go vnc.StartProxy(port)
}

func (vnc *VNCServer) Stop() {
	// Get logged in username
	username, err := GetLoggedInUser()
	if err != nil {
		log.Printf("[ERROR]: could not get logged in Username, reason: %v\n", err)
	}

	// Stop proxy
	if err := vnc.Proxy.Close(); err != nil {
		log.Printf("[ERROR]: could not stop VNC proxy, reason: %v\n", err)
	} else {
		log.Println("[INFO]: VNC proxy has been stopped")
	}

	// Remove PIN
	if err := vnc.RemovePIN(); err != nil {
		log.Printf("[ERROR]: could not remove vnc password, reason: %v", err)
	}

	// Stop gracefully VNC server
	if vnc.StopCommand != "" {
		if vnc.SystemctlCommand == "" {
			err := RunAsUser(vnc.StopCommand, vnc.StopCommandArgs, true)
			if err != nil {
				log.Printf("[ERROR]: VNC Stop error, %v\n", err)
				return
			}
		} else {
			command := fmt.Sprintf("%s %s", vnc.SystemctlCommand, strings.Join(vnc.StopCommandArgsFunc(username), " "))
			cmd := exec.Command("bash", "-c", command)
			if err := cmd.Run(); err != nil {
				log.Println("command to execute: ", vnc.SystemctlCommand, strings.Join(vnc.StopCommandArgsFunc(username), " "))
				log.Printf("[ERROR]: could not stop VNC server using %s, reason: %v", command, err)
			}
		}
	}

	log.Println("[INFO]: VNC server has been stopped")
}

func GetSupportedVNCServer(sid, proxyPort string) (*VNCServer, error) {
	supportedServers := map[string]VNCServer{
		"X11VNC": {
			StartCommand: `/usr/bin/x11vnc`,
			StartCommandArgsFunc: func(username string, port string, xauthority string) []string {
				cmd := []string{"-display", ":0", "-auth", xauthority, "-localhost", "-rfbauth", "/tmp/x11vncpasswd", "-forever", "-rfbport", port}
				return cmd
			},
			StopCommand:     "/usr/bin/x11vnc",
			StopCommandArgs: []string{"-R", "stop"},
			Configure: func() (string, error) {
				if isWaylandDisplayServer() {
					return "", errors.New("x11vnc cannot be used with Wayland display servers")
				}

				// Get the first available port for VNC server
				startingPort, err := strconv.Atoi(proxyPort)
				if err != nil {
					return "", err
				}

				for i := startingPort + 1; i < 65535; i++ {
					_, err := net.DialTimeout("tcp", ":"+strconv.Itoa(i), 5*time.Second)
					if err != nil {
						return strconv.Itoa(i), nil
					}
				}
				return "", errors.New("no free port available")
			},
			SavePIN: func(pin string) error {
				path := "/tmp/x11vncpasswd"

				if err := os.Remove(path); err != nil {
					log.Println("[INFO]: could not remove vnc password")
				}

				if err := RunAsUser(`/usr/bin/x11vnc`, []string{"-storepasswd", pin, path}, false); err != nil {
					return err
				}

				log.Println("[INFO]: PIN saved to ", path)
				return nil
			},
			RemovePIN: func() error {
				path := "/tmp/x11vncpasswd"

				if err := os.Remove(path); err != nil {
					return err
				}

				log.Println("[INFO]: PIN removed from ", path)
				return nil
			},
		},
		"GnomeRemoteDesktop": {
			StartCommand: "/usr/bin/grdctl",
			StartCommandArgsFunc: func(username string, port string, xauthority string) []string {
				cmd := []string{"shell", username + "@", "/usr/bin/systemctl --user enable --now gnome-remote-desktop.service"}
				return cmd
			},
			SystemctlCommand: "machinectl",
			StopCommand:      "machinectl",
			StopCommandArgsFunc: func(username string) []string {
				cmd := []string{"shell", username + "@", "/usr/bin/systemctl --user disable --now gnome-remote-desktop.service"}
				return cmd
			},
			Configure: func() (string, error) {
				err := RunAsUser("grdctl", []string{"vnc", "set-auth-method", "password"}, true)
				if err != nil {
					return "", errors.New("could not set set-auth-method")
				}

				err = RunAsUser("grdctl", []string{"vnc", "disable-view-only"}, true)
				if err != nil {
					return "", errors.New("could not set disable-view-only")
				}

				err = RunAsUser("grdctl", []string{"vnc", "enable"}, true)
				if err != nil {
					return "", errors.New("could not set enable grd")
				}
				err = RunAsUser("bash", []string{"-c", `gsettings set org.gnome.desktop.remote-desktop.vnc encryption "['none']"`}, true)
				if err != nil {
					log.Println("[INFO]: could not set vnc encryption to none")
				}

				return "5900", nil
			},
			RemovePIN: func() error {
				err := RunAsUser("grdctl", []string{"vnc", "disable"}, true)
				if err != nil {
					return errors.New("could not disable grd")
				}

				err = RunAsUser("grdctl", []string{"vnc", "enable-view-only"}, true)
				if err != nil {
					return errors.New("could not set enable-view-only")
				}

				err = RunAsUser("grdctl", []string{"vnc", "clear-password"}, true)
				if err != nil {
					return errors.New("could not clear password for grd")
				}

				err = RunAsUser("bash", []string{"-c", `gsettings reset org.gnome.desktop.remote-desktop.vnc encryption`}, true)
				if err != nil {
					log.Println("[INFO]: could not set vnc encryption to tls-anon")
				}

				return nil
			},
			SavePIN: func(pin string) error {
				err := RunAsUser("grdctl", []string{"vnc", "set-password", pin}, true)
				if err != nil {
					return errors.New("could not set password")
				}

				log.Println("[INFO]: gnome remote desktop password saved")
				return nil
			},
		},
	}

	for name, server := range supportedServers {
		if _, err := os.Stat(server.StartCommand); err == nil {
			if name == "X11VNC" && isWaylandDisplayServer() {
				continue
			}
			server.Name = name
			return &server, nil
		}
	}
	return nil, fmt.Errorf("no supported VNC server")
}

func (vnc *VNCServer) StartProxy(port string) {
	log.Printf("[INFO]: starting VNC proxy on port %s\n", vnc.ProxyPort)
	// Launch proxy only if port is available
	_, err := net.DialTimeout("tcp", ":"+vnc.ProxyPort, 5*time.Second)
	if err != nil {
		vncProxy := vncproxy.New(&vncproxy.Config{
			LogLevel: vncproxy.InfoLevel,
			TokenHandler: func(r *http.Request) (addr string, err error) {
				return ":" + port, nil
			},
		})
		vnc.Proxy.GET("/ws", func(ctx echo.Context) error {
			h := websocket.Handler(vncProxy.ServeWS)
			h.ServeHTTP(ctx.Response().Writer, ctx.Request())
			return nil
		})

		log.Println("[INFO]: NoVNC proxy server started")
		if err := vnc.Proxy.StartTLS(":"+vnc.ProxyPort, vnc.ProxyCert, vnc.ProxyKey); err != http.ErrServerClosed {
			log.Printf("[ERROR]: could not start VNC proxy\n, %v", err)
		}

	} else {
		log.Printf("[ERROR]: VNC proxy port %s is not available\n", vnc.ProxyPort)
	}
}

func isWaylandDisplayServer() bool {
	// Get XAUTHORITY
	xauthorityEnv, err := getXAuthority()
	if err != nil {
		log.Printf("[ERROR]: could not check if Wayland as I couldn't get XAUTHORITY env, reason: %v\n", err)
		return false
	}

	xauthority := strings.TrimPrefix(xauthorityEnv, "XAUTHORITY=")
	if strings.Contains(xauthority, "wayland") {
		return true
	}

	return false
}
