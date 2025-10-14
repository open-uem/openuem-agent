//go:build windows

package dsc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/open-uem/wingetcfg/wingetcfg"
)

func AddRegistryKey(key string, force bool) error {
	command := fmt.Sprintf("New-Item -Path '%s'", key)

	command += " -Force"

	return RunTaskWithLowPriority(command)
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

	return RunTaskWithLowPriority(command)
}

func UpdateRegistryKeyDefaultValue(path string, value string) error {
	command := fmt.Sprintf("Set-Item -Path '%s' -Value '%s'", path, value)

	return RunTaskWithLowPriority(command)
}

func RemoveRegistryKey(key string, recursive bool) error {
	command := fmt.Sprintf("Remove-Item -Path '%s'", key)

	if recursive {
		command += " -Recurse"
	}

	return RunTaskWithLowPriority(command)
}

func RemoveRegistryKeyValue(key string, name string) error {
	command := fmt.Sprintf("Remove-ItemProperty -Path '%s' -Name", key)

	return RunTaskWithLowPriority(command)
}
