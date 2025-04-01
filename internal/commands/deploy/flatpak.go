//go:build linux

package deploy

import (
	"log"
	"os/exec"
)

func InstallPackage(packageID string) error {
	log.Printf("[INFO]: received a request to install package %s", packageID)

	addRepoCmd := exec.Command("flatpak", "remote-add", "--if-not-exists", "flathub", "https://flathub.org/repo/flathub.flatpakrepo")
	err := addRepoCmd.Start()
	if err != nil {
		log.Printf("[ERROR]: could not start flatpak remote-add command, reason: %v", err)
		return err
	}

	cmd := exec.Command("flatpak", "install", "--assumeyes", "flathub", packageID)
	err = cmd.Start()
	if err != nil {
		log.Printf("[ERROR]: could not start flatpak command %v", err)
		return err
	}

	log.Printf("[INFO]: flatpak is installing an app, using command flatpak install --assumeyes flathub %s\n", packageID)
	err = cmd.Wait()
	if err != nil {
		log.Printf("[ERROR]: there was an error waiting for flatpak to finish %v", err)
		return err
	}
	log.Printf("[INFO]: flatpak has installed an application: %s", packageID)

	return nil
}

func UpdatePackage(packageID string) error {
	cmd := exec.Command("flatpak", "update", "--assumeyes", packageID)
	err := cmd.Start()
	if err != nil {
		log.Printf("[ERROR]: could not start flatpak command %v", err)
		return err
	}

	log.Printf("[INFO]: flatpak is updating an app, using command flatpak update --assumeyes %s\n", packageID)
	err = cmd.Wait()
	if err != nil {
		log.Printf("[ERROR]: there was an error waiting for flatpak to finish %v", err)
		return err
	}
	log.Println("[INFO]: flatpak has updated an application", packageID)

	return nil
}

func UninstallPackage(packageID string) error {
	cmd := exec.Command("flatpak", "remove", "--assumeyes", packageID)
	err := cmd.Start()
	if err != nil {
		log.Printf("[ERROR]: could not start flatpak command %v", err)
		return err
	}

	log.Printf("[INFO]: flatpak is removing an app, using command flatpak remove --assumeyes %s\n", packageID)
	err = cmd.Wait()
	if err != nil {
		log.Printf("[ERROR]: there was an error waiting for flatpak to finish %v", err)
		return err
	}
	log.Println("[INFO]: flatpak has removed an application", packageID)

	return nil
}
