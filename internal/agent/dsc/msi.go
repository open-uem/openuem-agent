//go:build windows

package dsc

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func InstallMSIPackage(path string, extraArguments string, logPath string) error {

	if path == "" {
		return errors.New("path cannot be empty")
	}

	// Installation path
	arguments := fmt.Sprintf("/I \"%s\" ", path)

	// Extra args
	arguments += extraArguments

	// Optional log
	if logPath != "" {
		arguments += fmt.Sprintf(" /log \"%s\"", logPath)
	}

	// Default arguments
	arguments += " /quiet /norestart"

	// Execute command
	command := fmt.Sprintf("Start-Process msiexec.exe -Wait -ArgumentList '%s'", arguments)
	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	return nil
}

func UninstallMSIPackage(path string, extraArguments string, logPath string) error {

	if path == "" {
		return errors.New("path cannot be empty")
	}

	// Installation path
	arguments := fmt.Sprintf("/X \"%s\" ", path)

	// Extra args
	arguments += extraArguments

	// Optional log
	if logPath != "" {
		arguments += fmt.Sprintf(" /log \"%s\"", logPath)
	}

	// Default arguments
	arguments += " /quiet /norestart"

	// Execute command
	command := fmt.Sprintf("Start-Process msiexec.exe -Wait -ArgumentList '%s'", arguments)
	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	return nil
}
