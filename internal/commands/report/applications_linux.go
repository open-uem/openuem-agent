//go:build linux

package report

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getApplicationsInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: applications info has been requested")
	}

	os := r.OS
	if os == "" {
		return errors.New("operating system version is empty")
	}

	desktopDirs := []string{
		"/usr/share/applications",
		"/usr/local/share/applications",
		"/opt",
		"/var/lib/snapd/desktop/applications",
		"/var/lib/flatpak/exports/share/applications",
	}

	for _, home := range findHomeDirs() {
		desktopDirs = append(desktopDirs, filepath.Join(home, ".local/share/applications"))
		desktopDirs = append(desktopDirs, filepath.Join(home, ".local/share/flatpak/exports/share/applications"))
	}

	// Get list of .desktop files
	desktopFiles := []string{}
	for _, d := range desktopDirs {
		desktopFiles = append(desktopFiles, findDesktopFilesInDir(d)...)
	}
	desktopFiles = slices.Compact(desktopFiles)

	appNames := []string{}

	// app.InstallDate not available for snap and .deb packages
	for _, p := range desktopFiles {
		if p != "" && strings.TrimSpace(p) != "Name" {
			app := openuem_nats.Application{}

			if strings.Contains(p, "flatpak/exports") {
				myApp, err := getFlatpakInfo(p)
				if err != nil {
					continue
				}
				app = *myApp
				if app.Name != "" && !slices.Contains(appNames, app.Name) {
					r.Applications = append(r.Applications, app)
					appNames = append(appNames, app.Name)
				}
			}

			if strings.Contains(p, "snapd/desktop") {
				myApp, err := getSnapInfo(p)
				if err != nil {
					continue
				}
				app = *myApp
				if app.Name != "" && !slices.Contains(appNames, app.Name) {
					r.Applications = append(r.Applications, app)
					appNames = append(appNames, app.Name)
				}
			}
			switch os {
			case "debian", "ubuntu", "linuxmint", "neon":
				if !strings.Contains(p, "flatpak/exports") && !strings.Contains(p, "snapd/desktop") {
					myApp, err := getDpkgInfo(p)
					if err != nil {
						continue
					}
					app = *myApp
					if app.Name != "" && !slices.Contains(appNames, app.Name) {
						r.Applications = append(r.Applications, app)
						appNames = append(appNames, app.Name)
					}
				}

			case "fedora", "opensuse-leap", "almalinux", "redhat", "rocky":
				if !strings.Contains(p, "flatpak/exports") && !strings.Contains(p, "snapd/desktop") {
					myApp, err := getRPMInfo(p)
					if err != nil {
						continue
					}
					app = *myApp
					if app.Name != "" && !slices.Contains(appNames, app.Name) {
						r.Applications = append(r.Applications, app)
						appNames = append(appNames, app.Name)
					}
				}
				// case "manjaro", "arch":
				// 	app.Name, app.Version, app.Publisher = getPackmanInfo(p)
				// 	if app.Name != "" {
				// 		r.Applications = append(r.Applications, app)
				// 	}
			}
		}
	}

	log.Println("[INFO]: desktop apps information has been retrieved from package manager")

	return nil
}

func getDpkgInfo(desktopFilePath string) (*openuem_nats.Application, error) {
	app := openuem_nats.Application{}

	desktopFile, err := os.ReadFile(desktopFilePath)
	if err != nil {
		return nil, err
	}

	// Get app's name from desktop file for a more precise name as suggested by @carlesgs
	reg := regexp.MustCompile(`Name=(.*?)\n`)
	matches := reg.FindAllStringSubmatch(string(desktopFile), -1)
	for _, v := range matches {
		app.Name = v[1]
		break
	}

	// Find the package name that provides .desktop file
	command := fmt.Sprintf(`dpkg -S %s 2>/dev/null | awk '{print $1}' | cut -f 1 -d ':'  | sort --unique`, desktopFilePath)
	out, err := exec.Command("bash", "-c", command).Output()
	if err != nil {
		return nil, errors.New("could not find deb package that creates .desktop file")
	}

	// Get information from the package
	command = fmt.Sprintf(`dpkg -s %s`, string(out))
	out, err = exec.Command("bash", "-c", command).Output()
	if err != nil {
		return nil, errors.New("could not find deb package")
	}

	reg = regexp.MustCompile(`Version: \s*(.*?)\s`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		app.Version = v[1]
		break
	}

	reg = regexp.MustCompile(`Original-Maintainer: \s*(.*?)\s<`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		app.Publisher = v[1]
		break
	}

	if app.Publisher == "" {
		reg = regexp.MustCompile(`Vendor: \s*(.*?)\s<`)
		matches = reg.FindAllStringSubmatch(string(out), -1)
		for _, v := range matches {
			app.Publisher = v[1]
			break
		}
	}

	if app.Publisher == "" {
		reg = regexp.MustCompile(`Maintainer: \s*(.*?)\s<`)
		matches = reg.FindAllStringSubmatch(string(out), -1)
		for _, v := range matches {
			app.Publisher = v[1]
			break
		}
	}

	return &app, nil
}

func getRPMInfo(desktopFilePath string) (*openuem_nats.Application, error) {
	app := openuem_nats.Application{}

	desktopFile, err := os.ReadFile(desktopFilePath)
	if err != nil {
		return nil, err
	}

	// Get app's name from desktop file for a more precise name as suggested by @carlesgs
	reg := regexp.MustCompile(`Name=(.*?)\n`)
	matches := reg.FindAllStringSubmatch(string(desktopFile), -1)
	for _, v := range matches {
		app.Name = v[1]
		break
	}

	// Find the package name that provides .desktop file
	command := fmt.Sprintf(`rpm -qf %s 2>/dev/null`, desktopFilePath)
	out, err := exec.Command("bash", "-c", command).Output()
	if err != nil {
		return nil, errors.New("could not find rpm package that creates .desktop file")
	}

	pkgName := strings.TrimSpace(string(out))
	command = fmt.Sprintf(`export LC_ALL=C && rpm -qi %s`, pkgName)
	out, err = exec.Command("bash", "-c", command).Output()
	if err != nil {
		return nil, errors.New("could not find rpm package")
	}

	reg = regexp.MustCompile(`Version     : \s*(.*?)\s`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		app.Version = v[1]
		break
	}

	reg = regexp.MustCompile(`Vendor      : \s*(.*?)\s`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		app.Publisher = v[1]
		break
	}

	reg = regexp.MustCompile(`Install Date:\s*(.*?)\n`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		t, err := time.Parse("Mon Jan 2 15:04:05 2006", v[1])
		if err == nil {
			app.InstallDate = t.Local().Format(time.DateOnly)
			break
		}
	}

	return &app, nil
}

// func getPackmanInfo(packageName string) (name string, version string, publisher string) {
// 	name = ""
// 	version = ""
// 	publisher = ""

// 	command := fmt.Sprintf("LANG=en_US.UTF-8 pacman -Si %s", packageName)
// 	out, err := exec.Command("bash", "-c", command).Output()
// 	if err != nil {
// 		return name, version, publisher
// 	}

// 	reg := regexp.MustCompile(`Name            : \s*(.*?)\s`)
// 	matches := reg.FindAllStringSubmatch(string(out), -1)
// 	for _, v := range matches {
// 		name = v[1]
// 		break
// 	}

// 	reg = regexp.MustCompile(`Version         : \s*(.*?)\s`)
// 	matches = reg.FindAllStringSubmatch(string(out), -1)
// 	for _, v := range matches {
// 		version = v[1]
// 		break
// 	}

// 	reg = regexp.MustCompile(`Packager        : \s*(.*?)\s<`)
// 	matches = reg.FindAllStringSubmatch(string(out), -1)
// 	for _, v := range matches {
// 		publisher = v[1]
// 		break
// 	}

// 	return name, version, publisher
// }

func findDesktopFilesInDir(root string) []string {
	var a []string
	filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if filepath.Ext(d.Name()) == ".desktop" {
			a = append(a, path)
		}
		return nil
	})
	return a
}

func getSnapInfo(desktopFilePath string) (*openuem_nats.Application, error) {
	app := openuem_nats.Application{}

	desktopFile, err := os.ReadFile(desktopFilePath)
	if err != nil {
		return nil, err
	}
	// Get app's name from desktop file for a more precise name as suggested by @carlesgs
	reg := regexp.MustCompile(`\nName=(.*?)\n`)
	matches := reg.FindAllStringSubmatch(string(desktopFile), -1)
	for _, v := range matches {
		app.Name = v[1]
		break
	}

	// Find the package name from snap info
	pkgName := filepath.Base(desktopFilePath)
	if app.Name == "" {
		app.Name = pkgName
	}
	out, err := exec.Command("snap", "info", pkgName).Output()
	if err != nil {
		return nil, errors.New("could not find snap package that creates .desktop file")
	}

	// Get package name
	reg = regexp.MustCompile(`name: \s*(.*?)\s`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		pkgName = v[1]
		break
	}

	// Find the package name from snap info
	command := fmt.Sprintf(`snap list %s | awk 'NR>1 {print $2}'`, pkgName)
	out, err = exec.Command("bash", "-c", command).Output()
	if err != nil {
		return nil, errors.New("could not find or parse snap package in snap list")
	}
	if string(out) != "" {
		app.Version = string(out)
	}

	app.Publisher = "Snap"

	return &app, nil
}

func getFlatpakInfo(desktopFilePath string) (*openuem_nats.Application, error) {
	app := openuem_nats.Application{}

	flatpak := strings.TrimSuffix(filepath.Base(desktopFilePath), ".desktop")

	desktopFile, err := os.ReadFile(desktopFilePath)
	if err != nil {
		return nil, err
	}

	// Get app's name from desktop file for a more precise name as suggested by @carlesgs
	reg := regexp.MustCompile(`Name=(.*?)\n`)
	matches := reg.FindAllStringSubmatch(string(desktopFile), -1)
	for _, v := range matches {
		app.Name = v[1]
		break
	}

	// Find the package info
	command := fmt.Sprintf(`export LC_ALL=C && flatpak info %s`, flatpak)
	out, err := exec.Command("bash", "-c", command).Output()
	if err != nil {
		return nil, errors.New("could not find flatpak package that creates .desktop file")
	}

	reg = regexp.MustCompile(`Version: \s*(.*?)\s`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		app.Version = v[1]
		break
	}

	app.Publisher = "Flatpak"

	reg = regexp.MustCompile(`Date: \s*(.*?)\s`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		t, err := time.Parse("2006-01-02", v[1])
		if err == nil {
			app.InstallDate = t.Local().Format(time.DateOnly)
			break
		}
	}

	return &app, nil
}

func findHomeDirs() []string {
	var a []string
	maxDepth := 2
	filepath.WalkDir("/home", func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}

		if d.IsDir() && strings.Count(path, string(os.PathSeparator)) > maxDepth {
			return fs.SkipDir
		}

		if d.IsDir() && d.Name() != "home" {
			a = append(a, path)
		}

		return nil
	})
	return a
}
