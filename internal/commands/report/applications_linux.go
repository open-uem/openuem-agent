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

	// TODO LINUX app.InstallDate
	for _, p := range desktopFiles {
		if p != "" && strings.TrimSpace(p) != "Name" {
			app := openuem_nats.Application{}
			switch os {
			case "debian", "ubuntu", "linuxmint", "neon":
				myApp, err := getDpkgInfo(p)
				if err != nil {
					continue
				}
				app = *myApp
				if app.Name != "" {
					r.Applications = append(r.Applications, app)
				}

			case "fedora", "opensuse-leap", "almalinux", "redhat", "rocky":
				app.Name, app.Version, app.Publisher = getRPMInfo(p)
				if app.Name != "" {
					r.Applications = append(r.Applications, app)
				}
			case "manjaro", "arch":
				app.Name, app.Version, app.Publisher = getPackmanInfo(p)
				if app.Name != "" {
					r.Applications = append(r.Applications, app)
				}
			}

		}
	}

	// // Now let's get flatpak apps
	// flatpakCommand := `flatpak list | grep system | awk -F'\t' '{print $1 "***" $3}'`
	// out, err = exec.Command("bash", "-c", flatpakCommand).Output()
	// if err != nil {
	// 	log.Println("[INFO]: could not get apps installed with flatpak")
	// } else {
	// 	for p := range strings.SplitSeq(string(out), "\n") {
	// 		if p != "" {
	// 			app := openuem_nats.Application{}
	// 			data := strings.Split(p, "***")
	// 			app.Name = strings.TrimSpace(data[0])
	// 			if len(data) > 1 {
	// 				app.Version = strings.TrimSpace(data[1])
	// 			} else {
	// 				app.Version = "-"
	// 			}
	// 			app.Publisher = "Flatpak"

	// 			r.Applications = append(r.Applications, app)
	// 		}
	// 	}
	// }

	// And snap - Duplicates ubuntu installs so we comment this snipper for the future
	// snapCommand := `snap list | grep -v 'Rev' | awk '{print $1 "---" $2 "---" $5}'`
	// out, err = exec.Command("bash", "-c", snapCommand).Output()
	// if err != nil {
	// 	log.Println("[INFO]: could not get apps installed with snap")
	// } else {
	// 	for p := range strings.SplitSeq(string(out), "\n") {
	// 		if p != "" {
	// 			app := openuem_nats.Application{}
	// 			data := strings.Split(p, "---")
	// 			app.Name = strings.TrimSpace(data[0])
	// 			if len(data) > 1 {
	// 				app.Version = strings.TrimSpace(data[1])
	// 			} else {
	// 				app.Version = "-"
	// 			}
	// 			if len(data) > 2 {
	// 				app.Publisher = strings.TrimSuffix(strings.TrimSpace(data[2]), "**")
	// 			} else {
	// 				app.Publisher = "Snap"
	// 			}

	// 			r.Applications = append(r.Applications, app)
	// 		}
	// 	}
	// }

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

	// Find the package name that provides .deskop file
	command := fmt.Sprintf(`dpkg -S %s 2>/dev/null | awk '{print $1}' | cut -f 1 -d ':'  | sort --unique`, desktopFilePath)
	out, err := exec.Command("bash", "-c", command).Output()
	if err != nil {
		return nil, errors.New("could not find deb package that creates .desktop file")
	}

	// Get information from the package
	command = fmt.Sprintf(`dpkg -s %s`, string(out))
	out, err = exec.Command("bash", "-c", command).Output()
	if err != nil {
		log.Println(err)
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

func getRPMInfo(packageName string) (name string, version string, publisher string) {
	name = ""
	version = ""
	publisher = ""

	out, err := exec.Command("rpm", "-qi", packageName).Output()
	if err != nil {
		return name, version, publisher
	}

	reg := regexp.MustCompile(`Name        : \s*(.*?)\s`)
	matches := reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		name = v[1]
		break
	}

	reg = regexp.MustCompile(`Version     : \s*(.*?)\s`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		version = v[1]
		break
	}

	reg = regexp.MustCompile(`Vendor      : \s*(.*?)\s`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		publisher = v[1]
		break
	}

	return name, version, publisher
}

func getPackmanInfo(packageName string) (name string, version string, publisher string) {
	name = ""
	version = ""
	publisher = ""

	command := fmt.Sprintf("LANG=en_US.UTF-8 pacman -Si %s", packageName)
	out, err := exec.Command("bash", "-c", command).Output()
	if err != nil {
		return name, version, publisher
	}

	reg := regexp.MustCompile(`Name            : \s*(.*?)\s`)
	matches := reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		name = v[1]
		break
	}

	reg = regexp.MustCompile(`Version         : \s*(.*?)\s`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		version = v[1]
		break
	}

	reg = regexp.MustCompile(`Packager        : \s*(.*?)\s<`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		publisher = v[1]
		break
	}

	return name, version, publisher
}

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
