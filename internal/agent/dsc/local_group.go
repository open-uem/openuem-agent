//go:build windows

package dsc

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func CreateLocalGroup(groupName string, description string) error {
	command := fmt.Sprintf("New-LocalGroup -Name '%s'", groupName)

	if description != "" {
		command += fmt.Sprintf(" -Description '%s'", description)
	}

	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	return nil
}

func RemoveLocalGroup(groupName string) error {
	command := fmt.Sprintf("Remove-LocalGroup -Name '%s'", groupName)

	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	return nil
}

func AddMembersToLocalGroup(groupName string, members string) error {
	command := fmt.Sprintf("Add-LocalGroupMember -Name '%s' -Member %s", groupName, members)

	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	return nil
}

func RemoveMembersFromLocalGroup(groupName string, members string) error {
	command := fmt.Sprintf("Remove-LocalGroupMember -Name '%s' -Member %s", groupName, members)

	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	return nil
}

func ExistsGroup(groupName string) bool {
	command := fmt.Sprintf("Get-LocalGroup -Name '%s'", groupName)

	cmd := exec.Command("PowerShell", "-command", command)
	err := cmd.Run()
	return err == nil
}
