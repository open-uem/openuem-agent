//go:build windows

package deploy

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"slices"
	"strings"

	"github.com/open-uem/nats"
	"github.com/open-uem/wingetcfg/wingetcfg"
)

func InstallPackage(action nats.DeployAction, keepUpdated bool, debug bool) (string, string, error) {
	if !IsValidPackageID(action.PackageId) || !IsValidPackageVersion(action.PackageVersion) {
		return "", "", fmt.Errorf("invalid package id %q or version %q", action.PackageId, action.PackageVersion)
	}

	wgPath, err := locateWinGet()
	if err != nil {
		log.Printf("[ERROR]: could not locate the winget.exe command %v", err)
		return "", "", err
	}

	log.Printf("[INFO]: received a request to install package %s using winget", action.PackageId)
	args := wingetInstallArgs(action.PackageId, action.PackageVersion)
	if debug {
		log.Printf("[DEBUG]: winget.exe is installing an app, using command %s %s\n", wgPath, strings.Join(args, " "))
	}

	output, err := runWinGet(wingetUpgradeTimeout, wgPath, args...)
	stdout := strings.Join(cleanWinGetOutput(output), "\n")

	// The package is already installed and there's no newer version: the requested
	// state is reached
	if errors.Is(err, ErrNoApplicableUpdate) {
		log.Printf("[INFO]: %s is already installed and up to date", action.PackageId)
		return stdout, "", nil
	}
	if err != nil {
		log.Printf("[ERROR]: winget.exe could not install %s, reason: %v", action.PackageId, err)
		reason := fmt.Sprintf("winget could not install %s: %v", action.PackageId, err)
		return stdout, reason + "\n" + outputTail(output, 10), fmt.Errorf("%s", reason)
	}

	log.Printf("[INFO]: winget.exe has installed an application: %s", action.PackageId)
	return stdout, "", nil
}

// UpdatePackage upgrades a package using winget. Machine-scoped packages are upgraded
// as SYSTEM, while user-scoped packages are upgraded in the session of every logged-in user.
func UpdatePackage(action nats.DeployAction) (string, string, error) {
	if !IsValidPackageID(action.PackageId) {
		return "", "", fmt.Errorf("invalid package id %q", action.PackageId)
	}

	if action.PackageScope == nats.PackageScopeUser {
		return updateUserPackage(action.PackageId)
	}

	wgPath, err := locateWinGet()
	if err != nil {
		log.Printf("[ERROR]: could not locate the winget.exe command %v", err)
		return "", "", err
	}

	log.Printf("[INFO]: winget.exe is upgrading the app %s\n", action.PackageId)
	output, err := runWinGet(wingetUpgradeTimeout, wgPath, wingetUpgradeArgs(action.PackageId, nats.PackageScopeMachine)...)

	// When no scope is requested, e.g. the package is updated on several computers from
	// the console, the package may have been installed for the logged-in users instead
	if errors.Is(err, errWinGetNoPackagesFound) {
		if action.PackageScope == "" {
			stdout, info, err := updateUserPackage(action.PackageId)
			if errors.Is(err, errNoLoggedInUsers) || errors.Is(err, errUserWinGetNotFound) {
				return output, "", ErrPackageNotInstalled
			}
			return stdout, info, err
		}
		return output, "", ErrPackageNotInstalled
	}

	if err != nil {
		if !errors.Is(err, ErrNoApplicableUpdate) {
			log.Printf("[ERROR]: winget.exe could not upgrade %s, reason: %v", action.PackageId, err)
		}
		return output, outputTail(output, 20), err
	}

	log.Printf("[INFO]: winget.exe has upgraded the app %s", action.PackageId)
	return output, "", nil
}

func UninstallPackage(action nats.DeployAction) (string, string, error) {
	if !IsValidPackageID(action.PackageId) {
		return "", "", fmt.Errorf("invalid package id %q", action.PackageId)
	}

	log.Printf("[INFO]: received a request to remove package %s using winget", action.PackageId)

	wgPath, err := locateWinGet()
	if err != nil {
		log.Printf("[ERROR]: could not locate the winget.exe command %v", err)
		return "", "", err
	}

	log.Printf("[INFO]: winget.exe is uninstalling the app %s\n", action.PackageId)
	output, err := runWinGet(wingetUpgradeTimeout, wgPath, wingetUninstallArgs(action.PackageId)...)
	stdout := strings.Join(cleanWinGetOutput(output), "\n")

	// Nothing installed matches: the package is already gone
	if errors.Is(err, errWinGetNoPackagesFound) {
		log.Printf("[INFO]: %s isn't installed, nothing to uninstall", action.PackageId)
		return stdout, "", nil
	}
	if err != nil {
		log.Printf("[ERROR]: winget.exe could not uninstall %s, reason: %v", action.PackageId, err)
		reason := fmt.Sprintf("winget could not uninstall %s: %v", action.PackageId, err)
		return stdout, reason + "\n" + outputTail(output, 10), fmt.Errorf("%s", reason)
	}

	log.Println("[INFO]: winget.exe has uninstalled an application")
	return stdout, "", nil
}

func locateWinGet() (string, error) {
	// We must find the location for winget.exe for local system user
	// Ref: https://github.com/microsoft/winget-cli/discussions/962#discussioncomment-1561274
	return newestWinGetPath("C:\\Program Files\\WindowsApps")
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
