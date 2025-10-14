//go:build windows

package dsc

import (
	"errors"
	"fmt"
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

	return RunTaskWithLowPriority(command)
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

	return RunTaskWithLowPriority(command)
}
