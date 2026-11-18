package deploy

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	appInstallerPrefix = "Microsoft.DesktopAppInstaller_"
	appInstallerSuffix = "_x64__8wekyb3d8bbwe"
)

// newestWinGetPath returns the winget.exe of the newest App Installer version in a
// WindowsApps folder. Versions are compared as numbers (1.26 is newer than 1.9), and
// versions without a usable winget.exe are skipped: while App Installer updates itself,
// the new version's folder may already exist with an empty winget.exe, and starting it
// fails with "%1 is not a valid Win32 application".
func newestWinGetPath(windowsApps string) (string, error) {
	entries, err := os.ReadDir(windowsApps)
	if err != nil {
		return "", err
	}

	newest, newestVersion := "", []int(nil)
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || !strings.HasPrefix(name, appInstallerPrefix) || !strings.HasSuffix(name, appInstallerSuffix) {
			continue
		}
		path := filepath.Join(windowsApps, name, "winget.exe")
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			continue
		}
		version := parseAppInstallerVersion(strings.TrimSuffix(strings.TrimPrefix(name, appInstallerPrefix), appInstallerSuffix))
		if newest == "" || compareVersionParts(version, newestVersion) > 0 {
			newest, newestVersion = path, version
		}
	}

	if newest == "" {
		return "", errors.New("desktopAppInstaller path not found")
	}
	return newest, nil
}

func parseAppInstallerVersion(s string) []int {
	parts := []int{}
	for _, p := range strings.Split(s, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = -1
		}
		parts = append(parts, n)
	}
	return parts
}

func compareVersionParts(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		x, y := 0, 0
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}
