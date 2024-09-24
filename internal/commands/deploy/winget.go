package deploy

import (
	"fmt"
	"io/fs"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
)

func installPackage(packageID string) error {
	wgPath, err := locateWinGet()
	if err != nil {
		log.Printf("[ERROR]: could not locate the winget.exe command %v", err)
		return err
	}

	cmd := exec.Command(wgPath, "install", packageID, "--scope", "machine", "--silent", "--accept-package-agreements", "--accept-source-agreements")
	err = cmd.Start()
	if err != nil {
		log.Printf("[ERROR]: could not start winget.exe command %v", err)
		return err
	}

	log.Println("[INFO]: winget.exe is installing an app", wgPath)
	err = cmd.Wait()
	if err != nil {
		log.Printf("[ERROR]: there was an error waiting for winget.exe to finish %v", err)
		return err
	}
	log.Println("[INFO]: winget.exe has installed an application", wgPath)

	// TODO Run a new report
	/* a.Run(true) */
	return nil
}

func uninstallPackage(packageID string) error {
	wgPath, err := locateWinGet()
	if err != nil {
		log.Printf("[ERROR]: could not locate the winget.exe command %v", err)
		return err
	}

	cmd := exec.Command(wgPath, "remove", packageID)
	err = cmd.Start()
	if err != nil {
		log.Printf("[ERROR]: could not start winget.exe command %v", err)
		return err
	}

	log.Println("[INFO]: winget.exe is uninstalling an app", wgPath)
	err = cmd.Wait()
	if err != nil {
		log.Printf("[ERROR]: there was an error waiting for winget.exe to finish %v", err)
		return err
	}
	log.Println("[INFO]: winget.exe has uninstalled an application", wgPath)

	// TODO Run a new report
	/* a.Run(true) */
	return nil
}

func locateWinGet() (string, error) {
	// We must find the location for winget.exe for local system user
	// Ref: https://github.com/microsoft/winget-cli/discussions/962#discussioncomment-1561274
	desktopAppInstallerPath := ""
	filepath.WalkDir("C:\\Program Files\\WindowsApps", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() && strings.HasPrefix(d.Name(), "Microsoft.DesktopAppInstaller_") && strings.HasSuffix(d.Name(), "_x64__8wekyb3d8bbwe") {
			desktopAppInstallerPath = path
		}
		return nil
	})

	if desktopAppInstallerPath == "" {
		return "", fmt.Errorf("desktopAppInstaller path not found")
	}

	// We must locate winget.exe
	wgPath, err := exec.LookPath(filepath.Join(desktopAppInstallerPath, "winget.exe"))
	if err != nil {
		return "", err
	}

	return wgPath, nil
}
