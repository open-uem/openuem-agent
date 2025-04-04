//go:build linux

package remotedesktop

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	"github.com/zcalusic/sysinfo"
)

func (rd *RemoteDesktopService) Start(pin string, notifyUser bool) {
	log.Println("[INFO]: a request to start a remote desktop service has been received")

	// Show PIN to user if needed
	if notifyUser {
		go func() {
			if err := notifyPINToUser(pin); err != nil {
				log.Printf("[ERROR]: could not show PIN message to user, reason: %v\n", err)
				return
			}
			log.Println("[INFO]: the PIN for remote assistance session should have been shown to the user in a browser")
		}()
	}

	// Configure Remote Desktop service
	if err := rd.Configure(); err != nil {
		log.Printf("[ERROR]: could not configure Remote Desktop service, reason: %v\n", err)
		return
	}
	log.Println("[INFO]: the remote desktop service has been configured")

	// Save PIN
	if err := rd.SavePIN(pin); err != nil {
		log.Printf("[ERROR]: could not save PIN before Remote Desktop service is started, reason: %v\n", err)
		return
	}

	// Get the first available port for VNC server
	vncPort := ""
	if rd.RequiresVNCProxy {
		vncPort = getFirstVNCAvailablePort()
		if vncPort == "" {
			log.Println("[ERROR]: could get a free port for VNC")
			return
		}
	}

	// Start Remote Desktop service
	go func() {
		log.Println("[INFO]: starting the remote desktop service...")
		if err := rd.StartService(vncPort); err != nil {
			log.Printf("[ERROR]: could not start Remote Desktop service, reason: %v", err)
			return
		}
	}()

	// Start VNC Proxy
	if rd.RequiresVNCProxy {
		go rd.StartVNCProxy(vncPort)
	}
}

func (rd *RemoteDesktopService) Stop() {
	if rd.RequiresVNCProxy {
		if err := rd.Proxy.Close(); err != nil {
			log.Printf("[ERROR]: could not stop VNC proxy, reason: %v\n", err)
		} else {
			log.Println("[INFO]: VNC proxy has been stopped")
		}
	}

	if err := rd.RemovePIN(); err != nil {
		log.Printf("[ERROR]: could not remove remote desktop credentials, reason: %v", err)
	}
	log.Println("[INFO]: the PIN for the remote desktop service has been removed")

	// Stop gracefully Remote Desktop service
	if err := rd.StopService(); err != nil {
		log.Printf("[ERROR]: could not stop the remote desktop service, reason: %v", err)
	}
	log.Println("[INFO]: the remote desktop service has been stopped")
}

func GetSupportedRemoteDesktopService(agentOS, sid, proxyPort string) (*RemoteDesktopService, error) {
	// Get logged in username
	username, err := runtime.GetLoggedInUser()
	if err != nil {
		return nil, err
	}

	supportedServers := map[string]RemoteDesktopService{
		"X11VNC": {
			RequiresVNCProxy: true,
			StartService: func(vncPort string) error {
				homeDir, _, _, err := getUserInfo(username)
				if err != nil {
					return err
				}
				openuemDir := filepath.Join(homeDir, ".openuem")
				path := filepath.Join(openuemDir, "x11vncpasswd")

				args := []string{"-display", ":0", "-auth", "guess", "-localhost", "-rfbauth", path, "-forever", "-rfbport", vncPort}
				if err := runtime.RunAsUser(username, "/usr/bin/x11vnc", args, true); err != nil {
					return err
				}
				return nil
			},
			StopService: func() error {
				args := []string{"-R", "stop"}
				if err := runtime.RunAsUser(username, "/usr/bin/x11vnc", args, true); err != nil {
					return err
				}
				return nil
			},
			Configure: func() error {
				return nil
			},
			SavePIN: func(pin string) error {
				homeDir, uid, gid, err := getUserInfo(username)
				if err != nil {
					return err
				}

				openuemDir := filepath.Join(homeDir, ".openuem")
				if err := createOpenUEMDir(openuemDir, uid, gid); err != nil {
					return err
				}

				path := filepath.Join(openuemDir, "x11vncpasswd")

				if err := os.Remove(path); err != nil {
					log.Println("[INFO]: could not remove vnc password")
				}

				if err := runtime.RunAsUser(username, `/usr/bin/x11vnc`, []string{"-storepasswd", pin, path}, false); err != nil {
					return err
				}

				log.Println("[INFO]: PIN saved to ", path)
				return nil
			},
			RemovePIN: func() error {
				homeDir, _, _, err := getUserInfo(username)
				if err != nil {
					log.Printf("[ERROR]: could not get user info, reason: %v", err)
					return err
				}

				openuemDir := filepath.Join(homeDir, ".openuem")
				if err := os.RemoveAll(openuemDir); err != nil {
					log.Println("[ERROR]: could not remove .openuem directory")
				}

				log.Println("[INFO]: PIN removed from ", openuemDir)
				return nil
			},
		},
		"GnomeRemoteDesktopRDP": {
			RequiresVNCProxy: false,
			StartService: func(vncPort string) error {
				command := fmt.Sprintf("machinectl shell %s@ /usr/bin/systemctl --user enable --now gnome-remote-desktop.service", username)
				cmd := exec.Command("bash", "-c", command)
				if err := cmd.Run(); err != nil {
					return err
				}
				return nil
			},
			StopService: func() error {
				command := fmt.Sprintf("machinectl shell %s@ /usr/bin/systemctl --user disable --now gnome-remote-desktop.service", username)
				cmd := exec.Command("bash", "-c", command)
				if err := cmd.Run(); err != nil {
					return err
				}
				return nil
			},
			Configure: func() error {
				homeDir, uid, gid, err := getUserInfo(username)
				if err != nil {
					return err
				}

				openuemDir := filepath.Join(homeDir, ".openuem")

				rdpCert := filepath.Join(openuemDir, "rdp-server.cer")
				rdpKey := filepath.Join(openuemDir, "rdp-server.key")

				if err := createOpenUEMDir(openuemDir, uid, gid); err != nil {
					return err
				}

				if err := copyCertFile("/etc/openuem-agent/certificates/server.cer", rdpCert, uid, gid); err != nil {
					return err
				}

				err = runtime.RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp set-tls-cert "+rdpCert)
				if err != nil {
					return errors.New("could not set set-tls-cert")
				}

				if err := copyCertFile("/etc/openuem-agent/certificates/server.key", rdpKey, uid, gid); err != nil {
					return err
				}

				err = runtime.RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp set-tls-key "+rdpKey)
				if err != nil {
					return errors.New("could not set set-tls-key")
				}

				err = runtime.RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp disable-view-only")
				if err != nil {
					return errors.New("could not set disable-view-only")
				}

				err = runtime.RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp enable")
				if err != nil {
					return errors.New("could not set enable grd")
				}

				return nil
			},
			RemovePIN: func() error {
				homeDir, _, _, err := getUserInfo(username)
				if err != nil {
					log.Printf("[ERROR]: could not get user info, reason: %v", err)
					return err
				}

				openuemDir := filepath.Join(homeDir, ".openuem")
				if err := os.RemoveAll(openuemDir); err != nil {
					log.Println("[ERROR]: could not remove .openuem directory")
				}

				err = runtime.RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp disable")
				if err != nil {
					log.Println("[ERROR]: could not disable grdctl")
				}

				err = runtime.RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp enable-view-only")
				if err != nil {
					log.Println("[ERROR]: could not set enable-view-only")
				}

				err = runtime.RunAsUserWithMachineCtl(username, "/usr/bin/grdctl rdp clear-credentials")
				if err != nil {
					log.Println("[ERROR]: could not clear password for grd")
				}

				return nil
			},
			SavePIN: func(pin string) error {
				command := fmt.Sprintf("/usr/bin/grdctl rdp set-credentials openuem %s", pin)
				err = runtime.RunAsUserWithMachineCtl(username, command)
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

func isWaylandDisplayServer() bool {
	// Get logged in username
	username, err := runtime.GetLoggedInUser()
	if err != nil {
		log.Printf("[ERROR]: could not get logged in Username, reason: %v\n", err)
		return false
	}

	_, uid, gid, err := getUserInfo(username)
	if err != nil {
		log.Printf("[ERROR]: could not get user info, reason: %v\n", err)
		return false
	}

	// Get XAUTHORITY
	xauthorityEnv, err := runtime.GetXAuthority(uint32(uid), uint32(gid))
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
		if _, err := os.Stat("/usr/bin/grdctl"); err == nil {
			return "GnomeRemoteDesktopRDP"
		}
		// Wayland requires grdctl for Gnome
		return ""
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

func getFirstVNCAvailablePort() string {
	for i := 5900; i < 65535; i++ {
		_, err := net.DialTimeout("tcp", ":"+strconv.Itoa(i), 5*time.Second)
		if err != nil {
			return strconv.Itoa(i)
		}
	}
	return ""
}

func notifyPINToUser(pin string) error {
	username, err := runtime.GetLoggedInUser()
	if err != nil {
		return err
	}

	notifyCommand := fmt.Sprintf("/opt/openuem-agent/bin/openuem-messenger info --message %s --type pin", pin)
	if err := runtime.RunAsUserWithMachineCtl(username, notifyCommand); err != nil {
		return err
	}

	return nil
}

func createOpenUEMDir(openuemDir string, uid, gid int) error {
	if err := os.MkdirAll(openuemDir, 0770); err != nil {
		log.Printf("[ERROR]: could not create openuem dir for current user, reason: %v", err)
		return err
	}

	if err := os.Chmod(openuemDir, 0770); err != nil {
		return err
	}

	if err := os.Chown(openuemDir, uid, gid); err != nil {
		return err
	}
	return nil
}

// "/etc/openuem-agent/certificates/server.cer"
func copyCertFile(src, dst string, uid, gid int) error {
	if err := copyFileContents(src, dst); err != nil {
		return err
	}

	if err := os.Chmod(dst, 0600); err != nil {
		return err
	}

	if err := os.Chown(dst, uid, gid); err != nil {
		return err
	}
	return nil
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

func getUserInfo(username string) (homedir string, uid int, gid int, err error) {
	u, err := user.Lookup(username)
	if err != nil {
		return "", -1, -1, err
	}

	uid, err = strconv.Atoi(u.Uid)
	if err != nil {
		return "", -1, -1, err
	}

	gid, err = strconv.Atoi(u.Gid)
	if err != nil {
		return "", -1, -1, err
	}

	return u.HomeDir, uid, gid, nil
}
