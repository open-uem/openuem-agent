//go:build darwin

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
	"strconv"
	"time"

	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	openuem_utils "github.com/open-uem/utils"
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

	vncPort := "5900"

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
	supportedServers := map[string]RemoteDesktopService{
		// Reference: https://community.hetzner.com/tutorials/how-to-enable-vnc-on-macos-via-ssh
		"MacOS Remote Management": {
			RequiresVNCProxy: true,
			StartService: func(vncPort string) error {
				command := "/System/Library/CoreServices/RemoteManagement/ARDAgent.app/Contents/Resources/kickstart -activate"
				cmd := exec.Command("bash", "-c", command)
				if err := cmd.Run(); err != nil {
					return err
				}

				command = "/System/Library/CoreServices/RemoteManagement/ARDAgent.app/Contents/Resources/kickstart -configure -allowAccessFor -allUsers -privs -all"
				cmd = exec.Command("bash", "-c", command)
				if err := cmd.Run(); err != nil {
					return err
				}

				command = "/System/Library/CoreServices/RemoteManagement/ARDAgent.app/Contents/Resources/kickstart -configure -clientopts -setvnclegacy -vnclegacy yes"
				cmd = exec.Command("bash", "-c", command)
				if err := cmd.Run(); err != nil {
					return err
				}

				command = "/System/Library/CoreServices/RemoteManagement/ARDAgent.app/Contents/Resources/kickstart -restart -agent -console"
				cmd = exec.Command("bash", "-c", command)
				if err := cmd.Run(); err != nil {
					return err
				}
				return nil
			},
			StopService: func() error {
				command := "/System/Library/CoreServices/RemoteManagement/ARDAgent.app/Contents/Resources/kickstart -stop -agent -console"
				cmd := exec.Command("bash", "-c", command)
				if err := cmd.Run(); err != nil {
					return err
				}

				command = "/System/Library/CoreServices/RemoteManagement/ARDAgent.app/Contents/Resources/kickstart -deactivate"
				cmd = exec.Command("bash", "-c", command)
				if err := cmd.Run(); err != nil {
					return err
				}

				return nil
			},
			Configure: func() error {
				return nil
			},
			RemovePIN: func() error {
				pin, err := openuem_utils.GenerateRandomPIN()
				command := fmt.Sprintf("/System/Library/CoreServices/RemoteManagement/ARDAgent.app/Contents/Resources/kickstart -configure -clientopts -setvncpw -vncpw %s", pin)
				cmd := exec.Command("bash", "-c", command)
				if err := cmd.Run(); err != nil {
					return errors.New("could not save random VNC credentials")
				}

				if err != nil {
					log.Printf("[ERROR]: could not generate random PIN, reason: %v\n", err)
					return err
				}

				return nil
			},
			SavePIN: func(pin string) error {
				command := fmt.Sprintf("/System/Library/CoreServices/RemoteManagement/ARDAgent.app/Contents/Resources/kickstart -configure -clientopts -setvncpw -vncpw %s", pin)
				cmd := exec.Command("bash", "-c", command)
				if err := cmd.Run(); err != nil {
					return errors.New("could not set VNC credentials")
				}

				log.Println("[INFO]: VNC credentials saved")
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
	return "MacOS Remote Management"
}

func GetAgentOS() string {
	return "macOS"
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

	// Reference: https://stackoverflow.com/questions/5588064/how-do-i-make-a-mac-terminal-pop-up-alert-applescript
	args := []string{"-e", fmt.Sprintf(`display alert "OpenUEM Remote Assistance" message "PIN: %s"`, pin)}
	if err := runtime.RunAsUser(username, "osascript", args, false); err != nil {
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
