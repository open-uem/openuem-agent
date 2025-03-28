package vnc

import (
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

func RunAsUser(cmdPath string, args []string, env bool) error {
	cmd := exec.Command(cmdPath, args...)

	log.Println("[INFO]: command to execute is ", cmdPath, args)

	uid, gid, err := getLoggedInUserIDs()
	if err != nil {
		return err
	}

	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return err
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
	}

	// Run command adding env variables
	if env {
		// Get DISPLAY environment variable
		display, err := getDisplay()
		if err != nil {
			return err
		}

		// Get XAUTHORITY environment variable
		xauthority, err := getXAuthority()
		if err != nil {
			return err
		}

		// Chrome, Firefox in Linux need env variables like USER, DISPLAY, XAUTHORITY...
		cmd.Env = append(os.Environ(), "USER="+u.Username, "HOME="+u.HomeDir, strings.TrimSpace(display), strings.TrimSpace(xauthority))
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

func getLoggedInUser() (string, error) {
	username := ""

	cmd := "loginctl list-sessions --no-legend | grep seat0 | awk '{ print $2,$3 }'"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return "", err
	}

	loginCtlOut := string(out)
	if loginCtlOut == "" {
		return "", nil
	}

	for _, u := range strings.Split(loginCtlOut, "\n") {
		userInfo := strings.Split(u, " ")
		if len(userInfo) == 2 {
			uid, err := strconv.Atoi(userInfo[0])
			if err != nil {
				log.Printf("[ERROR]: could not get uid from loginctl, %s", u)
				continue
			}
			if uid < 1000 {
				log.Printf("[INFO]: uid is lower than 1000, %s it's not a regular user", userInfo[1])
				continue
			}
			username = userInfo[1]
		}
	}

	return username, nil
}

func getLoggedInUserIDs() (int, int, error) {
	username, err := getLoggedInUser()
	if err != nil {
		log.Println("[ERROR]: could not get current logged in username")
		return -1, -1, err
	}

	u, err := user.Lookup(username)
	if err != nil {
		log.Println("[ERROR]: could not find user by username ", username)
		return -1, -1, err
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		log.Println("[ERROR]: could not convert uid to int ", u.Uid)
		return -1, -1, err
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		log.Println("[ERROR]: could not convert gid to int ", u.Gid)
		return -1, -1, err
	}

	return uid, gid, err
}

// Get XAUTHORITY environment variable
func getXAuthority() (string, error) {
	uid, gid, err := getLoggedInUserIDs()
	if err != nil {
		return "", err
	}

	// Ref: https://unix.stackexchange.com/questions/429092/what-is-the-best-way-to-find-the-current-display-and-xauthority-in-non-interacti
	envCmd := exec.Command("bash", "-c", `ps -u $(id -u) -o pid= | xargs -I{} cat /proc/{}/environ 2>/dev/null | tr '\0' '\n' | grep -m1 '^XAUTHORITY='`)
	envCmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
	}
	envOut, err := envCmd.Output()
	if err != nil {
		log.Println("[ERROR]: could not execute systemctl --user show-environment")
		return "", err
	}
	xauthority := string(envOut)

	return xauthority, nil
}

// Get DISPLAY environment variable
func getDisplay() (string, error) {
	uid, gid, err := getLoggedInUserIDs()
	if err != nil {
		return "", err
	}

	// Ref: https://unix.stackexchange.com/questions/429092/what-is-the-best-way-to-find-the-current-display-and-xauthority-in-non-interacti
	envCmd := exec.Command("bash", "-c", `ps -u $(id -u) -o pid= | xargs -I{} cat /proc/{}/environ 2>/dev/null | tr '\0' '\n' | grep -m1 '^DISPLAY='`)
	envCmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
	}
	envOut, err := envCmd.Output()
	if err != nil {
		log.Println("[ERROR]: could not execute systemctl --user show-environment")
		return "", err
	}
	xauthority := string(envOut)

	return xauthority, nil
}
