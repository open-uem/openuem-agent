//go:build darwin

package deploy

import (
	"log"
	"runtime"
	"strings"

	"github.com/open-uem/nats"
	openuem_runtime "github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func InstallPackage(action nats.DeployAction, keepUpdated bool, debug bool) (string, string, error) {
	var args []string

	isCask := false
	if strings.HasPrefix(action.PackageId, "cask-") {
		isCask = true
		action.PackageId = strings.TrimPrefix(action.PackageId, "cask-")
	}
	if action.PackageBrewType == "cask" {
		isCask = true
	}

	log.Printf("[INFO]: received a request to install package %s using brew", action.PackageId)

	brewPath := getBrewPath()

	if isCask {
		args = []string{"install", "--cask", action.PackageId}
	} else {
		args = []string{"install", action.PackageId}
	}

	username, err := openuem_runtime.GetLoggedInUser()
	if err != nil {
		log.Printf("[ERROR]: could not find the logged in user, reason %v", err)
		return "", "", err
	}

	out, err := openuem_runtime.RunAsUserWithOutput(username, brewPath, args, false)
	if err != nil {
		log.Printf("[ERROR]: found and error with brew install command, reason %s", string(out))
		return "", string(out), err
	}

	log.Printf("[INFO]: brew has installed an application: %s", action.PackageId)

	return "", "", nil
}

func UpdatePackage(action nats.DeployAction) (string, string, error) {
	var args []string

	isCask := false

	if strings.HasPrefix(action.PackageId, "cask-") {
		isCask = true
		action.PackageId = strings.TrimPrefix(action.PackageId, "cask-")
	}
	if action.PackageBrewType == "cask" {
		isCask = true
	}
	log.Printf("[INFO]: received a request to upgrade package %s", action.PackageId)

	brewPath := getBrewPath()

	if isCask {
		args = []string{"upgrade", "--force", "--cask", action.PackageId}
	} else {
		args = []string{"upgrade", "--force", action.PackageId}
	}

	username, err := openuem_runtime.GetLoggedInUser()
	if err != nil {
		log.Printf("[ERROR]: could not find the logged in user, reason %v", err)
		return "", "", err
	}

	out, err := openuem_runtime.RunAsUserWithOutput(username, brewPath, args, false)
	if err != nil {
		log.Printf("[ERROR]: found and error with brew upgrade command, reason %s", string(out))
		return "", string(out), err
	}

	log.Printf("[INFO]: brew has updated an application: %s", action.PackageId)

	return "", "", nil
}

func UninstallPackage(action nats.DeployAction) (string, string, error) {
	var args []string

	isCask := false

	if strings.HasPrefix(action.PackageId, "cask-") {
		isCask = true
		action.PackageId = strings.TrimPrefix(action.PackageId, "cask-")
	}
	if action.PackageBrewType == "cask" {
		isCask = true
	}
	log.Printf("[INFO]: received a request to remove package %s using brew", action.PackageId)

	brewPath := getBrewPath()

	if isCask {
		args = []string{"uninstall", "--force", "--cask", action.PackageId}
	} else {
		args = []string{"uninstall", "--force", action.PackageId}
	}

	username, err := openuem_runtime.GetLoggedInUser()
	if err != nil {
		log.Printf("[ERROR]: could not find the logged in user, reason %v", err)
		return "", "", err
	}

	out, err := openuem_runtime.RunAsUserWithOutput(username, brewPath, args, false)
	if err != nil {
		log.Printf("[ERROR]: found and error with brew remove command, reason %s", string(out))
		return "", string(out), err
	}

	log.Printf("[INFO]: brew has removed an application: %s", action.PackageId)

	return "", "", nil
}

func getBrewPath() string {
	brewPath := "/opt/homebrew/bin/brew"
	if runtime.GOARCH == "amd64" {
		brewPath = "/usr/local/bin/brew"
	}
	return brewPath
}
