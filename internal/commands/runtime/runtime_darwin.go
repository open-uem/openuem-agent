//go:build darwin

package runtime

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

func RunAsUser(username, cmdPath string, args []string, env bool) error {
	cmd := exec.Command(cmdPath, args...)

	u, err := user.Lookup(username)
	if err != nil {
		return err
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
	}

	// Run command adding env variables
	if env {
		// Get DISPLAY environment variable
		display, err := GetDisplay(uint32(uid), uint32(gid))
		if err != nil {
			return err
		}

		// Get XAUTHORITY environment variable
		xauthority, err := GetXAuthority(uint32(uid), uint32(gid))
		if err != nil {
			return err
		}

		// Chrome, Firefox in Linux need env variables like USER, DISPLAY, XAUTHORITY...
		cmd.Env = append(os.Environ(), "USER="+u.Username, "HOME="+u.HomeDir, strings.TrimSpace(display), strings.TrimSpace(xauthority))
	}

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func RunAsUserWithMachineCtl(username, myCmd string) error {
	command := fmt.Sprintf("machinectl shell %s@ %s", username, myCmd)
	cmd := exec.Command("bash", "-c", command)
	if err := cmd.Run(); err != nil {
		log.Printf("[ERROR]: could not run command %s, reason: %v", command, err)
	}
	return nil
}

func GetLoggedInUser() (string, error) {
	cmd := "stat -f '%Su' /dev/console"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// Get XAUTHORITY environment variable
func GetXAuthority(uid, gid uint32) (string, error) {
	// Ref: https://unix.stackexchange.com/questions/429092/what-is-the-best-way-to-find-the-current-display-and-xauthority-in-non-interacti
	envCmd := exec.Command("bash", "-c", `ps -u $(id -u) -o pid= | xargs -I{} cat /proc/{}/environ 2>/dev/null | tr '\0' '\n' | grep -m1 '^XAUTHORITY='`)
	envCmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
	}
	envOut, err := envCmd.Output()
	if err != nil {
		log.Println("[ERROR]: could not execute bash script to get XAuthority")
		return "", err
	}
	xauthority := string(envOut)

	return xauthority, nil
}

// Get DISPLAY environment variable
func GetDisplay(uid, gid uint32) (string, error) {

	// Ref: https://unix.stackexchange.com/questions/429092/what-is-the-best-way-to-find-the-current-display-and-xauthority-in-non-interacti
	envCmd := exec.Command("bash", "-c", `ps -u $(id -u) -o pid= | xargs -I{} cat /proc/{}/environ 2>/dev/null | tr '\0' '\n' | grep -m1 '^DISPLAY='`)
	envCmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uid, Gid: gid},
	}
	envOut, err := envCmd.Output()
	if err != nil {
		log.Println("[ERROR]: could not execute bash script to get Display")
		return "", err
	}
	xauthority := string(envOut)

	return xauthority, nil
}
