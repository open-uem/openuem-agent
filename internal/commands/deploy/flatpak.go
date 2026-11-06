//go:build linux

package deploy

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
)

const flatpakListTimeout = 3 * time.Minute

func InstallPackage(action nats.DeployAction, keepUpdated bool, debug bool) (string, string, error) {
	log.Printf("[INFO]: received a request to install package %s", action.PackageId)

	cmd := "flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo"
	if err := exec.Command("bash", "-c", cmd).Run(); err != nil {
		log.Printf("[ERROR]: could not start flatpak remote-add command, reason: %v", err)
		return "", "", err
	}

	packageRef := action.PackageId
	if action.PackageBranch != "" {
		packageRef = fmt.Sprintf("%s//%s", action.PackageId, action.PackageBranch)
	}

	if out, err := runtime.RunAsUserWithOutput("root", "flatpak", []string{"install", "--noninteractive", "--assumeyes", "flathub", packageRef}, true); err != nil {
		log.Printf("[ERROR]: found and error with flatpak install command, reason %v", err)
		return "", string(out), err
	}

	log.Printf("[INFO]: flatpak has installed an application: %s", packageRef)

	return "", "", nil
}

func UpdatePackage(action nats.DeployAction) (string, string, error) {
	log.Printf("[INFO]: received a request to update package %s", action.PackageId)

	cmd := "flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo"

	if err := exec.Command("bash", "-c", cmd).Run(); err != nil {
		log.Printf("[ERROR]: could not start flatpak remote-add command, reason: %v", err)
		return "", "", err
	}

	packageRef := action.PackageId
	if action.PackageBranch != "" {
		packageRef = fmt.Sprintf("%s//%s", action.PackageId, action.PackageBranch)
	}

	username := "root"
	args := []string{"update", "--noninteractive", "--assumeyes", packageRef}
	if action.PackageScope == nats.PackageScopeUser {
		loggedInUser, err := runtime.GetLoggedInUser()
		if err != nil || loggedInUser == "" {
			return "", "", errors.New("user-scoped packages can only be updated while their user is logged in")
		}
		username = loggedInUser
		args = []string{"update", "--user", "--noninteractive", "--assumeyes", packageRef}
	}

	if out, err := runtime.RunAsUserWithOutput(username, "flatpak", args, true); err != nil {
		log.Printf("[ERROR]: found and error with flatpak update command, reason %v", err)
		return "", string(out), err
	}

	log.Printf("[INFO]: flatpak has updated an application %s", action.PackageId)

	return "", "", nil
}

func UninstallPackage(action nats.DeployAction) (string, string, error) {
	log.Printf("[INFO]: received a request to remove package %s using flatpak", action.PackageId)

	cmd := "flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo"
	if err := exec.Command("bash", "-c", cmd).Run(); err != nil {
		log.Printf("[ERROR]: could not start flatpak remote-add command, reason: %v", err)
		return "", "", err
	}

	packageRef := action.PackageId
	if action.PackageBranch != "" {
		packageRef = fmt.Sprintf("%s//%s", action.PackageId, action.PackageBranch)
	}

	if out, err := runtime.RunAsUserWithOutput("root", "flatpak", []string{"remove", "--noninteractive", "--assumeyes", packageRef}, true); err != nil {
		log.Printf("[ERROR]: found and error with flatpak remove command, reason %v", err)
		return "", string(out), err
	}

	log.Printf("[INFO]: flatpak has removed an application %s", packageRef)

	return "", "", nil
}

// ListAvailableUpdates returns the flatpak apps with an available update, both from the
// system installation and from the installation of the logged-in user
func ListAvailableUpdates() ([]nats.AvailableUpdate, error) {
	if _, err := exec.LookPath("flatpak"); err != nil {
		return []nats.AvailableUpdate{}, nil
	}

	updates, err := listFlatpakUpdates("root", "--system", nats.PackageScopeMachine)
	if err != nil {
		return nil, err
	}

	username, err := runtime.GetLoggedInUser()
	if err != nil || username == "" {
		return updates, nil
	}

	userUpdates, err := listFlatpakUpdates(username, "--user", nats.PackageScopeUser)
	if err != nil {
		log.Printf("[ERROR]: could not list flatpak updates for user %s, reason: %v", username, err)
		return updates, nil
	}
	for i := range userUpdates {
		userUpdates[i].Users = []string{username}
	}

	return append(updates, userUpdates...), nil
}

func listFlatpakUpdates(username, installation, scope string) ([]nats.AvailableUpdate, error) {
	remoteUpdates, err := runtime.RunAsUserWithOutputAndTimeout(username, "flatpak", append([]string{"remote-ls", installation, "--updates"}, FlatpakUpdateColumns...), true, flatpakListTimeout)
	if err != nil {
		return nil, fmt.Errorf("could not list flatpak updates, reason: %v", err)
	}

	installed, err := runtime.RunAsUserWithOutputAndTimeout(username, "flatpak", append([]string{"list", installation}, FlatpakInstalledColumns...), true, flatpakListTimeout)
	if err != nil {
		return nil, fmt.Errorf("could not list flatpak apps, reason: %v", err)
	}

	return ParseFlatpakUpdates(string(remoteUpdates), string(installed), scope), nil
}
