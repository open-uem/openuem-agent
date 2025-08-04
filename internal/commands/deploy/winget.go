//go:build windows

package deploy

import (
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

func InstallPackage(packageID string) error {
	wgPath, err := locateWinGet()
	if err != nil {
		log.Printf("[ERROR]: could not locate the winget.exe command %v", err)
		return err
	}

	log.Printf("[INFO]: received a request to install package %s", packageID)

	cmd := exec.Command(wgPath, "install", packageID, "--scope", "machine", "--silent", "--accept-package-agreements", "--accept-source-agreements")
	err = cmd.Start()
	if err != nil {
		log.Printf("[ERROR]: could not start winget.exe command %v", err)
		return err
	}

	err = runtime.SetPriorityWindows(cmd.Process.Pid, windows.IDLE_PRIORITY_CLASS)
	if err != nil {
		log.Println("[ERROR]: could not change process priority")
	}

	log.Printf("[INFO]: winget.exe is installing an app, using command %s %s %s %s %s %s %s %s\n", wgPath, "install", packageID, "--scope", "machine", "--silent", "--accept-package-agreements", "--accept-source-agreements")
	err = cmd.Wait()
	if err != nil {
		log.Printf("[ERROR]: there was an error waiting for winget.exe to finish %v", err)
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
		log.Printf("[ERROR]: there was an error waiting for winget.exe to finish %v", err)
		return err
	}
	log.Println("[INFO]: winget.exe has upgraded an application", wgPath)

	return nil
}

func UninstallPackage(packageID string) error {
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
		log.Printf("[ERROR]: there was an error waiting for winget.exe to finish %v", err)
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

func RemovePackagesFromCfg(cfg *wingetcfg.WinGetCfg, exclusions []string) error {
	if len(exclusions) == 0 {
		return nil
	}

	validResources := []*wingetcfg.WinGetResource{}
	for _, r := range cfg.Properties.Resources {
		if r.Resource == wingetcfg.WinGetPackageResource {
			if !slices.Contains(exclusions, r.Settings["id"].(string)) || (slices.Contains(exclusions, r.Settings["id"].(string)) && r.Settings["Ensure"].(string) == "Absent") {
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
