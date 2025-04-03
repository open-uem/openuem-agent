//go:build linux

package deploy

import (
	"fmt"
	"log"
	"os/exec"
)

func InstallPackage(packageID string) error {
	log.Printf("[INFO]: received a request to install package %s", packageID)

	cmd := "flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo"
	if err := exec.Command("bash", "-c", cmd).Run(); err != nil {
		log.Printf("[ERROR]: could not start flatpak remote-add command, reason: %v", err)
		return err
	}

	cmd = fmt.Sprintf("flatpak install --noninteractive --assumeyes flathub %s", packageID)
	if err := exec.Command("bash", "-c", cmd).Run(); err != nil {
		log.Printf("[ERROR]: found and error with flatpak install command, reason %v", err)
		return err
	}

	log.Printf("[INFO]: flatpak has installed an application: %s", packageID)

	return nil
}

func UpdatePackage(packageID string) error {
	log.Printf("[INFO]: received a request to update package %s", packageID)

	cmd := "flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo"

	if err := exec.Command("bash", "-c", cmd).Run(); err != nil {
		log.Printf("[ERROR]: could not start flatpak remote-add command, reason: %v", err)
		return err
	}

	cmd = fmt.Sprintf("flatpak update --noninteractive --assumeyes %s", packageID)
	if err := exec.Command("bash", "-c", cmd).Run(); err != nil {
		log.Printf("[ERROR]: found and error with flatpak update command, reason %v", err)
		return err
	}

	log.Println("[INFO]: flatpak has updated an application", packageID)

	return nil
}

func UninstallPackage(packageID string) error {
	log.Printf("[INFO]: received a request to remove package %s", packageID)

	cmd := "flatpak remote-add --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo"
	if err := exec.Command("bash", "-c", cmd).Run(); err != nil {
		log.Printf("[ERROR]: could not start flatpak remote-add command, reason: %v", err)
		return err
	}

	cmd = fmt.Sprintf("flatpak remove --noninteractive --assumeyes %s", packageID)
	if err := exec.Command("bash", "-c", cmd).Run(); err != nil {
		log.Printf("[ERROR]: found and error with flatpak remove command, reason %v", err)
		return err
	}

	log.Println("[INFO]: flatpak has removed an application", packageID)

	return nil
}
