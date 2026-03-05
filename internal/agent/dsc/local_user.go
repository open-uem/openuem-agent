//go:build windows

package dsc

import (
	"bytes"
	"fmt"
	"os/exec"
)

func CreateLocalUser(username string, password string, comment string, fullName string, disabled bool, passwordChangeNotAllowed bool, passwordNeverExpires bool, changePasswordAtLogon bool) (string, string, error) {

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

	if _, _, err := RunTaskWithLowPriority(command); err != nil {
		return "", "", err
	}

	if changePasswordAtLogon {
		var stderr bytes.Buffer
		var stdout bytes.Buffer

		args := []string{"user", username, "/logonpasswordchg:yes"}
		cmd := exec.Command("net", args...)
		cmd.Stderr = &stderr
		cmd.Stdout = &stdout

		if err := cmd.Run(); err != nil {
			return "", "", err
		}

		return stdout.String(), stderr.String(), nil
	}

	return "", "", nil
}

func DeleteLocalUser(username string) (string, string, error) {
	command := fmt.Sprintf("Remove-LocalUser -Name %s", username)

	return RunTaskWithLowPriority(command)
}
