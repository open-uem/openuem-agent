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
	sudoArgs := []string{"-u", username}
	sudoArgs = append(sudoArgs, cmdPath)
	sudoArgs = append(sudoArgs, args...)

	cmd := exec.Command("sudo", sudoArgs...)

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

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func RunAsUserWithOutput(username, cmdPath string, args []string, env bool) ([]byte, error) {
	sudoArgs := []string{"-u", username}
	sudoArgs = append(sudoArgs, cmdPath)
	sudoArgs = append(sudoArgs, args...)

	cmd := exec.Command("sudo", sudoArgs...)

	u, err := user.Lookup(username)
	if err != nil {
		return nil, err
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, err
	}

	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil, err
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return output, err
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

func RunAsUserInBackground(username, cmdPath string, args []string, env bool) error {
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
		cmd.Env = append(os.Environ(), "USER="+u.Username, "HOME="+u.HomeDir)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[ERROR]: run as user %s found an err: %v", username, err)
		return err
	}

	return nil
}
