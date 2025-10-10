//go:build windows

package dsc

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func CreateLocalUser(username string, password string, comment string, fullName string, disabled bool, passwordChangeNotAllowed bool, passwordNeverExpires bool, changePasswordAtLogon bool) error {

	command := fmt.Sprintf("New-LocalUser -Name '%s'", username)

	if password != "" {
		command += fmt.Sprintf(" -Password %s", fmt.Sprintf("( ConvertTo-SecureString '%s' -AsPlainText -Force )", password))
	}

	if comment != "" {
		command += fmt.Sprintf(" -Description '%s'", comment)
	}

	if fullName != "" {
		command += fmt.Sprintf(" -FullName '%s'", fullName)
	}

	if disabled {
		command += " -Disabled"
	}

	if passwordNeverExpires {
		command += " -PasswordNeverExpires"
	}

	if passwordChangeNotAllowed {
		command += " -UserMayNotChangePassword"
	}

	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	if changePasswordAtLogon {
		args := []string{"user", username, "/logonpasswordchg:yes"}
		cmd := exec.Command("net", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			errMessages := strings.Split(string(out), ".")
			return errors.New(errMessages[0])
		}
	}

	return nil
}

func DeleteLocalUser(username string) error {
	command := fmt.Sprintf("Remove-LocalUser -Name %s", username)

	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	return nil
}
