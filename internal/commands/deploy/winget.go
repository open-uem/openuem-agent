//go:build windows

package deploy

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	"github.com/open-uem/wingetcfg/wingetcfg"
	"golang.org/x/sys/windows"
)

func InstallPackage(packageID string, version string, keepUpdated bool, debug bool) error {
	var cmd *exec.Cmd
	var out bytes.Buffer

	wgPath, err := locateWinGet()
	if err != nil {
		log.Printf("[ERROR]: could not locate the winget.exe command %v", err)
		return err
	}

	log.Printf("[INFO]: received a request to install package %s using winget", packageID)

	if version != "" {
		cmd = exec.Command(wgPath, "install", packageID, "--version", version, "--scope", "machine", "--silent", "--accept-package-agreements", "--accept-source-agreements")
	} else {
		cmd = exec.Command(wgPath, "install", packageID, "--scope", "machine", "--silent", "--accept-package-agreements", "--accept-source-agreements")
	}

	cmd.Stderr = &out

	err = cmd.Start()
	if err != nil {
		log.Printf("[ERROR]: could not start winget.exe command %v", err)
		return err
	}

	err = runtime.SetPriorityWindows(cmd.Process.Pid, windows.IDLE_PRIORITY_CLASS)
	if err != nil {
		log.Println("[ERROR]: could not change process priority")
	}

	if debug {
		log.Printf("[DEBUG]: winget.exe is installing an app, using command %s %s %s %s %s %s %s %s\n", wgPath, "install", packageID, "--scope", "machine", "--silent", "--accept-package-agreements", "--accept-source-agreements")
	}
	err = cmd.Wait()
	if err != nil {
		errCode := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(err.Error(), "exit status "))), "0X", "0x")
		errMessage, ok := wingetcfg.ErrorCodes[errCode]
		if !ok {
			errMessage = err.Error() + " " + out.String()
		}

		// Package is already installed and no applicable update is found
		if errCode == "0x8A15002B" {
			log.Printf("[INFO]: %s cannot be updated. %s", packageID, errMessage)
			if !keepUpdated {
				return nil
			}
		}

		log.Printf("[ERROR]: there was an error running winget.exe: %v", errMessage)
		return err
	}
	log.Printf("[INFO]: winget.exe has installed an application: %s", packageID)

	return nil
}

func UpdatePackage(packageID string) error {
	wgPath, err := locateWinGet()
	if err != nil {
		log.Printf("[ERROR]: could not locate the winget.exe command %v", err)
		return err
	}

	cmd := exec.Command(wgPath, "upgrade", packageID, "--scope", "machine", "--silent", "--accept-package-agreements", "--accept-source-agreements")
	err = cmd.Start()
	if err != nil {
		log.Printf("[ERROR]: could not start winget.exe command %v", err)
		return err
	}

	err = runtime.SetPriorityWindows(cmd.Process.Pid, windows.IDLE_PRIORITY_CLASS)
	if err != nil {
		log.Println("[ERROR]: could not change process priority")
	}

	log.Printf("[INFO]: winget.exe is upgrading an app, using command %s %s %s %s %s %s %s %s\n", wgPath, "install", packageID, "--scope", "machine", "--silent", "--accept-package-agreements", "--accept-source-agreements")
	err = cmd.Wait()
	if err != nil {
		errCode := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(err.Error(), "exit status "))), "0X", "0x")
		errMessage, ok := wingetcfg.ErrorCodes[errCode]
		if !ok {
			errMessage = err.Error()
		}

		log.Printf("[ERROR]: there was an error waiting for winget.exe to finish %v", errMessage)
		return err
	}
	log.Println("[INFO]: winget.exe has upgraded an application", wgPath)

	return nil
}

func UninstallPackage(packageID string) error {
	log.Printf("[INFO]: received a request to remove package %s using brew", packageID)

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

	err = runtime.SetPriorityWindows(cmd.Process.Pid, windows.IDLE_PRIORITY_CLASS)
	if err != nil {
		log.Println("[ERROR]: could not change process priority")
	}

	log.Printf("[INFO]: winget.exe is uninstalling the app %s\n", packageID)
	err = cmd.Wait()
	if err != nil {
		errCode := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(err.Error(), "exit status "))), "0X", "0x")
		errMessage, ok := wingetcfg.ErrorCodes[errCode]
		if !ok {
			errMessage = err.Error()
		}

		if errCode == "0x8A150014" {
			log.Printf("[INFO]: %s cannot be uninstalled. %s", packageID, errMessage)
			return nil
		}

		log.Printf("[ERROR]: there was an error waiting for winget.exe to finish %v", errMessage)
		return err
	}
	log.Println("[INFO]: winget.exe has uninstalled an application")

	return nil
}

func locateWinGet() (string, error) {
	// We must find the location for winget.exe for local system user
	// Ref: https://github.com/microsoft/winget-cli/discussions/962#discussioncomment-1561274
	desktopAppInstallerPath := ""
	if err := filepath.WalkDir("C:\\Program Files\\WindowsApps", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() && strings.HasPrefix(d.Name(), "Microsoft.DesktopAppInstaller_") && strings.HasSuffix(d.Name(), "_x64__8wekyb3d8bbwe") {
			desktopAppInstallerPath = path
		}
		return nil
	}); err != nil {
		return "", err
	}

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

func GetExplicitelyDeletedPackages(deployments []string, installed string) []string {
	deleted := []string{}

	for _, d := range deployments {
		if !strings.Contains(installed, d) {
			deleted = append(deleted, d)
		}
	}

	return deleted
}

func GetWinGetInstalledPackagesList() (string, error) {
	wgPath, err := locateWinGet()
	if err != nil {
		log.Printf("[ERROR]: could not locate the winget.exe command %v", err)
		return "", err
	}

	out, err := exec.Command(wgPath, "list").Output()
	if err != nil {
		return "", err
	}

	return string(out), nil
}

func RemovePackagesFromCfg(cfg *wingetcfg.WinGetCfg, explicitelyDeleted []string, exclusions []string, installed string, debug bool) error {
	if debug {
		log.Println("[DEBUG]: Installed packages ", installed)
	}

	validResources := []*wingetcfg.WinGetResource{}
	for _, r := range cfg.Properties.Resources {
		if r.Resource == wingetcfg.WinGetPackageResource {
			isPackageExcluded := slices.Contains(exclusions, r.Settings["id"].(string))
			isPackageExplicitelyDeleted := slices.Contains(explicitelyDeleted, r.Settings["id"].(string))
			isAlreadyInstalled := strings.Contains(installed, r.Settings["id"].(string))
			isInstallAction := r.Settings["Ensure"].(string) == "Present"

			if debug {
				log.Printf("[DEBUG]: Package %s, Is installed? %t, Excluded? %t, Explicitely Deleted %t,", r.Settings["id"], isAlreadyInstalled, isPackageExcluded, isPackageExplicitelyDeleted)
			}

			if !isPackageExcluded && !isPackageExplicitelyDeleted &&
				((isInstallAction && !isAlreadyInstalled) || (!isInstallAction && isAlreadyInstalled)) {
				validResources = append(validResources, r)
			}

		} else {
			validResources = append(validResources, r)
		}
	}

	cfg.Properties.Resources = validResources

	return nil
}

type PowerShellTask struct {
	ID        string
	Script    string
	RunConfig string
}

func RemovePowershellScriptsFromCfg(cfg *wingetcfg.WinGetCfg) map[string]PowerShellTask {
	scripts := map[string]PowerShellTask{}
	validResources := []*wingetcfg.WinGetResource{}
	for _, r := range cfg.Properties.Resources {
		if r.Resource == wingetcfg.OpenUEMPowershell {
			script, ok := r.Settings["Script"]
			if ok {
				name, ok := r.Settings["Name"]
				if ok {
					id, ok := r.Settings["ID"]
					if ok {
						scriptRun, ok := r.Settings["ScriptRun"]
						if ok {
							scripts[name.(string)] = PowerShellTask{
								Script:    script.(string),
								RunConfig: scriptRun.(string),
								ID:        id.(string),
							}
						} else {
							scripts[name.(string)] = PowerShellTask{
								Script:    script.(string),
								RunConfig: "once",
								ID:        id.(string),
							}
						}
					}

				}
			}
		} else {
			validResources = append(validResources, r)
		}
	}

	cfg.Properties.Resources = validResources

	return scripts
}
