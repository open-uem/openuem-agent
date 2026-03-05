//go:build darwin

package deploy

import (
	"log"
	"runtime"
	"strings"

	openuem_runtime "github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func InstallPackage(packageID string, version string, keepUpdated bool, debug bool) (string, string, error) {
	var args []string

	isCask := false
	if strings.HasPrefix(packageID, "cask-") {
		isCask = true
		packageID = strings.TrimPrefix(packageID, "cask-")
	}
	log.Printf("[INFO]: received a request to install package %s using brew", packageID)

	brewPath := getBrewPath()

	if isCask {
		args = []string{"install", "--cask", packageID}
	} else {
		args = []string{"install", packageID}
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

	log.Printf("[INFO]: brew has installed an application: %s", packageID)

	return "", "", nil
}

func UpdatePackage(packageID string) (string, string, error) {
	var args []string

	isCask := false

	if strings.HasPrefix(packageID, "cask-") {
		isCask = true
		packageID = strings.TrimPrefix(packageID, "cask-")
	}
	log.Printf("[INFO]: received a request to upgrade package %s", packageID)

	brewPath := getBrewPath()

	if isCask {
		args = []string{"upgrade", "--force", "--cask", packageID}
	} else {
		args = []string{"upgrade", "--force", packageID}
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

	log.Printf("[INFO]: brew has updated an application: %s", packageID)

	return "", "", nil
}

func UninstallPackage(packageID string) (string, string, error) {
	var args []string

	isCask := false

	if strings.HasPrefix(packageID, "cask-") {
		isCask = true
		packageID = strings.TrimPrefix(packageID, "cask-")
	}
	log.Printf("[INFO]: received a request to remove package %s using brew", packageID)

	brewPath := getBrewPath()

	if isCask {
		args = []string{"uninstall", "--force", "--cask", packageID}
	} else {
		args = []string{"uninstall", "--force", packageID}
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

	log.Printf("[INFO]: brew has removed an application: %s", packageID)

	return "", "", nil
}

func getBrewPath() string {
	brewPath := "/opt/homebrew/bin/brew"
	if runtime.GOARCH == "amd64" {
		brewPath = "/usr/local/bin/brew"
	}
	return brewPath
}
