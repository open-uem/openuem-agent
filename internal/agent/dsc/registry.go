//go:build windows

package dsc

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
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

func AddOrEditRegistryValue(path string, name string, propertyType string, value string, force bool) error {
	command := fmt.Sprintf("New-ItemProperty -Path '%s' -Name '%s' -PropertyType '%s'", path, name, propertyType)

	if propertyType != wingetcfg.RegistryValueTypeMultistring {
		command += fmt.Sprintf(" -Value '%s'", value)
	} else {
		values := []string{}
		for v := range strings.SplitSeq(value, "\n") {
			values = append(values, fmt.Sprintf("'%s'", v))
		}
		command += fmt.Sprintf(" -Value @(%s)", strings.Join(values, ", "))
	}

	if force {
		command += " -Force"
	}

	log.Println(command)

	cmd := exec.Command("PowerShell", "-command", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessages := strings.Split(string(out), ".")
		return errors.New(errMessages[0])
	}

	// 	RegistryValueTypeString       string = "String"
	// RegistryValueTypeBinary       string = "Binary"
	// RegistryValueTypeDWord        string = "DWord"
	// RegistryValueTypeQWord        string = "QWord"
	// RegistryValueTypeMultistring  string = "MultiString"
	// RegistryValueTypeExpandString string = "ExpandString"

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
