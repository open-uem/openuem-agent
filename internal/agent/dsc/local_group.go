//go:build windows

package dsc

import (
	"fmt"
	"os/exec"
)

func CreateLocalGroup(groupName string, description string) error {
	command := fmt.Sprintf("New-LocalGroup -Name '%s'", groupName)

	if description != "" {
		command += fmt.Sprintf(" -Description '%s'", description)
	}

	return RunTaskWithLowPriority(command)
}

func RemoveLocalGroup(groupName string) error {
	command := fmt.Sprintf("Remove-LocalGroup -Name '%s'", groupName)

	return RunTaskWithLowPriority(command)
}

func AddMembersToLocalGroup(groupName string, members string) error {
	command := fmt.Sprintf("Add-LocalGroupMember -Name '%s' -Member %s", groupName, members)

	return RunTaskWithLowPriority(command)
}

func RemoveMembersFromLocalGroup(groupName string, members string) error {
	command := fmt.Sprintf("Remove-LocalGroupMember -Name '%s' -Member %s", groupName, members)

	return RunTaskWithLowPriority(command)
}

func ExistsGroup(groupName string) bool {
	command := fmt.Sprintf("Get-LocalGroup -Name '%s'", groupName)

	cmd := exec.Command("PowerShell", "-command", command)
	err := cmd.Run()
	return err == nil
}
