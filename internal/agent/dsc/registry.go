//go:build windows

package dsc

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/open-uem/wingetcfg/wingetcfg"
)

func AddRegistryKey(key string, force bool) (string, string, error) {
	key, err := translatePath(key)
	if err != nil {
		return "", "", err
	}

	command := fmt.Sprintf("New-Item -Path '%s'", key)

	command += " -Force"

	return RunTaskWithLowPriority(command)
}

func AddOrEditRegistryValue(path string, name string, propertyType string, value string, hex bool, force bool) (string, string, error) {
	path, err := translatePath(path)
	if err != nil {
		return "", "", err
	}

	command := fmt.Sprintf("New-ItemProperty -Path '%s' -Name '%s' -PropertyType '%s'", path, name, propertyType)

	switch propertyType {
	case wingetcfg.RegistryValueTypeDWord, wingetcfg.RegistryValueTypeQWord:
		if hex {
			i, err := strconv.ParseInt(value, 16, 64)
			if err != nil {
				return "", "", err
			}
			command += fmt.Sprintf(" -Value %d", i)
		} else {
			i, err := strconv.Atoi(value)
			if err != nil {
				return "", "", err
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

func UpdateRegistryKeyDefaultValue(path string, value string) (string, string, error) {
	path, err := translatePath(path)
	if err != nil {
		return "", "", err
	}

	command := fmt.Sprintf("Set-Item -Path '%s' -Value '%s'", path, value)

	return RunTaskWithLowPriority(command)
}

func RemoveRegistryKey(key string, recursive bool) (string, string, error) {
	key, err := translatePath(key)
	if err != nil {
		return "", "", err
	}

	command := fmt.Sprintf("Remove-Item -Path '%s'", key)

	if recursive {
		command += " -Recurse"
	}

	return RunTaskWithLowPriority(command)
}

func RemoveRegistryKeyValue(key string, name string) (string, string, error) {

	key, err := translatePath(key)
	if err != nil {
		return "", "", err
	}

	command := fmt.Sprintf("Remove-ItemProperty -Path '%s' -Name", key)

	return RunTaskWithLowPriority(command)
}

func translatePath(path string) (string, error) {
	if strings.Contains(path, "HKEY_CLASSES_ROOT") {
		return strings.ReplaceAll(path, "HKEY_CLASSES_ROOT", "HKCR:"), nil
	}

	if strings.Contains(path, "HKEY_CURRENT_USER") {
		return strings.ReplaceAll(path, "HKEY_CURRENT_USER", "HKCU:"), nil
	}

	if strings.Contains(path, "HKEY_LOCAL_MACHINE") {
		return strings.ReplaceAll(path, "HKEY_LOCAL_MACHINE", "HKLM:"), nil
	}

	if strings.Contains(path, "HKEY_USERS") {
		return strings.ReplaceAll(path, "HKEY_USERS", "HKU:"), nil
	}

	if strings.Contains(path, "HKEY_CURRENT_CONFIG") {
		return strings.ReplaceAll(path, "HKEY_CURRENT_CONFIG", "HKCC:"), nil
	}

	if strings.Contains(path, "HKCR:") || strings.Contains(path, "HKCU:") || strings.Contains(path, "HKLM:") || strings.Contains(path, "HKU:") || strings.Contains(path, "HKCC:") {
		return path, nil
	}

	return "", errors.New("no valid root key found in path")
}
