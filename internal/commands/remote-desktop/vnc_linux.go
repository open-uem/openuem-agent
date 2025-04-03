//go:build linux

package remotedesktop

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/evangwt/go-vncproxy"
	"github.com/labstack/echo/v4"
	"github.com/zcalusic/sysinfo"
	"golang.org/x/net/websocket"
)

func (rd *RemoteDesktopService) Start(pin string, notifyUser bool) {
	// Get logged in username
	username, err := GetLoggedInUser()
	if err != nil {
		log.Printf("[ERROR]: could not get logged in username, reason: %v\n", err)
	}

	log.Println("[INFO]: a request to start a remote desktop service has been received")

	// Show PIN to user if needed
	if notifyUser {
		go func() {
			notifyCommand := fmt.Sprintf("/opt/openuem-agent/bin/openuem-messenger info --message %s --type pin", pin)
			if err := RunAsUserWithMachineCtl(username, notifyCommand); err != nil {
				log.Printf("[ERROR]: could not show PIN message to user, reason: %v\n", err)
				return
			}

			log.Println("[INFO]: the PIN for remote assistance session should have been shown to the user in a browser")
		}()
	}

	// Configure Remote Desktop service
	port, err := rd.Configure()
	if err != nil {
		log.Printf("[ERROR]: could not configure Remote Desktop service, reason: %v\n", err)
		return
	}
	log.Println("[INFO]: the remote desktop service has been configured")

	// Save PIN
	if err := rd.SavePIN(pin); err != nil {
		log.Printf("[ERROR]: could not save PIN before Remote Desktop service is started, reason: %v\n", err)
		return
	}

	// Start Remote Desktop service
	go func() {
		xauthorityEnv, err := getXAuthority()
		if err != nil {
			log.Printf("[ERROR]: could not get XAUTHORITY env, reason: %v\n", err)
		}

		xauthority := strings.TrimPrefix(xauthorityEnv, "XAUTHORITY=")

		if rd.SystemctlCommand == "" {
			if err := RunAsUser(rd.StartCommand, rd.StartCommandArgsFunc(username, port, xauthority), true); err != nil {
				log.Printf("[ERROR]: could not start Remote Desktop service, reason: %v", err)
			} else {
				log.Println("[INFO]: the remote desktop service should have been started")
			}
		} else {
			command := fmt.Sprintf("%s %s", rd.SystemctlCommand, strings.Join(rd.StartCommandArgsFunc(username, port, xauthority), " "))
			cmd := exec.Command("bash", "-c", command)
			if err := cmd.Run(); err != nil {
				log.Printf("[ERROR]: could not start Remote Desktop service using %s, reason: %v", command, err)
			} else {
				log.Println("[INFO]: the remote desktop service should have been started")
			}
		}
	}()

	// Start VNC Proxy
	if rd.RequiresVNCProxy {
		go rd.StartVNCProxy(port)
		log.Printf("[INFO]: the VNC proxy should have been started and listed at port %s", port)
	}
}

func (rd *RemoteDesktopService) Stop() {
	// Get logged in username
	username, err := GetLoggedInUser()
	if err != nil {
		log.Printf("[ERROR]: could not get logged in Username, reason: %v\n", err)
	}

	// Stop proxy
	if rd.RequiresVNCProxy {
		if err := rd.Proxy.Close(); err != nil {
			log.Printf("[ERROR]: could not stop VNC proxy, reason: %v\n", err)
		} else {
			log.Println("[INFO]: VNC proxy has been stopped")
		}
	}

	// Remove PIN
	if err := rd.RemovePIN(); err != nil {
		log.Printf("[ERROR]: could not remove vnc password, reason: %v", err)
	}
	log.Println("[INFO]: the PIN for the remote desktop service has been removed")

	// Stop gracefully Remote Desktop service
	if rd.StopCommand != "" {
		if rd.SystemctlCommand == "" {
			err := RunAsUser(rd.StopCommand, rd.StopCommandArgs, true)
			if err != nil {
				log.Printf("[ERROR]: Remote Desktop service stop error, %v\n", err)
				return
			}
		} else {
			command := fmt.Sprintf("%s %s", rd.SystemctlCommand, strings.Join(rd.StopCommandArgsFunc(username), " "))
			cmd := exec.Command("bash", "-c", command)
			if err := cmd.Run(); err != nil {
				log.Printf("[ERROR]: could not stop Remote Desktop service using %s, reason: %v", command, err)
			}
		}
	}

	log.Println("[INFO]: the Remote Desktop service has been stopped")
}

func GetSupportedRemoteDesktopService(agentOS, sid, proxyPort string) (*RemoteDesktopService, error) {
	supportedServers := map[string]RemoteDesktopService{
		"X11VNC": {
			RequiresVNCProxy: true,
			StartCommand:     `/usr/bin/x11vnc`,
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
				startingPort := 5900
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
		"GnomeRemoteDesktopVNC": {
			RequiresVNCProxy: true,
			StartCommand:     "/usr/bin/grdctl",
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
		"GnomeRemoteDesktopRDP": {
			RequiresVNCProxy: false,
			StartCommand:     "/usr/bin/grdctl",
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
				username, err := GetLoggedInUser()
				if err != nil {
					log.Println("[ERROR]: could not get current logged in username")
					return "", err
				}

				u, err := user.Lookup(username)
				if err != nil {
					return "", errors.New("could not find username")
				}

				uid, err := strconv.Atoi(u.Uid)
				if err != nil {
					return "", err
				}

				gid, err := strconv.Atoi(u.Gid)
				if err != nil {
					return "", err
				}

				openuemDir := filepath.Join(u.HomeDir, ".openuem")
				if err := os.MkdirAll(openuemDir, 0770); err != nil {
					log.Printf("[ERROR]: could not create openuem dir for current user, reason: %v", err)
					return "", err
				}

				rdpCert := filepath.Join(openuemDir, "rdp-server.cer")
				rdpKey := filepath.Join(openuemDir, "rdp-server.key")

				if err := copyFileContents("/etc/openuem-agent/certificates/server.cer", rdpCert); err != nil {
					return "", err
				}

				if err := os.Chmod(rdpCert, 0600); err != nil {
					return "", err
				}

				if err := os.Chown(rdpCert, uid, gid); err != nil {
					return "", err
				}

				err = RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp set-tls-cert "+rdpCert)
				if err != nil {
					return "", errors.New("could not set set-tls-cert")
				}

				if err := copyFileContents("/etc/openuem-agent/certificates/server.key", rdpKey); err != nil {
					return "", err
				}

				if err := os.Chmod(rdpKey, 0600); err != nil {
					return "", err
				}

				if err := os.Chown(rdpKey, uid, gid); err != nil {
					return "", err
				}

				err = RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp set-tls-key "+rdpKey)
				if err != nil {
					return "", errors.New("could not set set-tls-key")
				}

				err = RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp disable-view-only")
				if err != nil {
					return "", errors.New("could not set disable-view-only")
				}

				err = RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp enable")
				if err != nil {
					return "", errors.New("could not set enable grd")
				}

				return "3389", nil
			},
			RemovePIN: func() error {
				username, err := GetLoggedInUser()
				if err != nil {
					log.Println("[ERROR]: could not get current logged in username")
					return err
				}

				u, err := user.Lookup(username)
				if err != nil {
					log.Println("[ERROR]: could not find user by username")
				} else {
					openuemDir := filepath.Join(u.HomeDir, ".openuem")
					if err := os.RemoveAll(openuemDir); err != nil {
						log.Println("[ERROR]: could not remove .openuem directory")
					}
				}

				err = RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp disable")
				if err != nil {
					log.Println("[ERROR]: could not disable grdctl")
				}

				err = RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp enable-view-only")
				if err != nil {
					log.Println("[ERROR]: could not set enable-view-only")
				}

				err = RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp clear-credentials")
				if err != nil {
					log.Println("[ERROR]: could not clear password for grd")
				}

				return nil
			},
			SavePIN: func(pin string) error {
				username, err := GetLoggedInUser()
				if err != nil {
					log.Println("[ERROR]: could not get current logged in username")
					return err
				}

				err = RunAsUserWithMachineCtl(username, fmt.Sprintf("/usr/bin/grdctl rdp set-credentials openuem %s", pin))
				if err != nil {
					return errors.New("could not set rdp credentials")
				}

				log.Println("[INFO]: gnome remote desktop credentials saved")
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

func (rd *RemoteDesktopService) StartVNCProxy(port string) {
	log.Printf("[INFO]: starting VNC proxy on port %s\n", rd.ProxyPort)
	// Launch proxy only if port is available
	_, err := net.DialTimeout("tcp", ":"+rd.ProxyPort, 5*time.Second)
	if err != nil {
		vncProxy := vncproxy.New(&vncproxy.Config{
			LogLevel: vncproxy.InfoLevel,
			TokenHandler: func(r *http.Request) (addr string, err error) {
				return ":" + port, nil
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

func GetSupportedRemoteDesktop(agentOS string) string {

	// Check if we're using a Wayland Display Server
	if isWaylandDisplayServer() {
		if agentOS == "debian" || agentOS == "ubuntu" {
			if _, err := os.Stat("/usr/bin/grdctl"); err == nil {
				return "GnomeRemoteDesktopRDP"
			}
			// Wayland requires grdctl
			return ""
		} else {
			if _, err := os.Stat("/usr/bin/grdctl"); err == nil {
				return "GnomeRemoteDesktopVNC"
			}
		}
	} else {
		if _, err := os.Stat("/usr/bin/x11vnc"); err == nil {
			return "X11VNC"
		}
	}

	return ""
}

func GetAgentOS() string {
	var si sysinfo.SysInfo
	si.GetSysInfo()
	return si.OS.Vendor
}

func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}
