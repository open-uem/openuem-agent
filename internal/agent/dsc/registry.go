//go:build windows

package dsc

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/open-uem/wingetcfg/wingetcfg"
)

func AddRegistryKey(key string) error {
	command := fmt.Sprintf("New-Item -Path '%s'", key)

	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	return nil
}

func AddOrEditRegistryValue(path string, name string, propertyType string, value string, hex bool, force bool) error {
	command := fmt.Sprintf("New-ItemProperty -Path '%s' -Name '%s' -PropertyType '%s'", path, name, propertyType)

	switch propertyType {
	case wingetcfg.RegistryValueTypeDWord, wingetcfg.RegistryValueTypeQWord:
		if hex {
			i, err := strconv.ParseInt(value, 16, 64)
			if err != nil {
				return err
			}
			command += fmt.Sprintf(" -Value %d", i)
		} else {
			i, err := strconv.Atoi(value)
			if err != nil {
				return err
			}
			command += fmt.Sprintf(" -Value %d", i)
		}
	case wingetcfg.RegistryValueTypeMultistring:
		values := []string{}
		for v := range strings.SplitSeq(value, "\n") {
			values = append(values, fmt.Sprintf("'%s'", v))
		}
		command += fmt.Sprintf(" -Value @(%s)", strings.Join(values, ", "))
	case wingetcfg.RegistryValueTypeString, wingetcfg.RegistryValueTypeExpandString:
		command += fmt.Sprintf(" -Value '%s'", value)
	}

	if force {
		command += " -Force"
	}

	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	return nil
}

func UpdateRegistryKeyDefaultValue(path string, value string) error {
	command := fmt.Sprintf("Set-Item -Path '%s' -Value '%s'", path, value)

	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	return nil
}

func RemoveRegistryKey(key string, recursive bool) error {
	command := fmt.Sprintf("Remove-Item -Path '%s'", key)

	if recursive {
		command += " -Recurse"
	}

	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	return nil
}

func RemoveRegistryKeyValue(key string, name string) error {
	command := fmt.Sprintf("Remove-ItemProperty -Path '%s' -Name", key)

	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	return nil
}
