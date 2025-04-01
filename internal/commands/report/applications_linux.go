//go:build linux

package report

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getApplicationsInfo(debug bool) error {
	command := ""

	if debug {
		log.Println("[DEBUG]: applications info has been requested")
	}

	os := r.OS
	if os == "" {
		return errors.New("operating system version is empty")
	}

	switch os {
	case "debian", "ubuntu", "linuxmint":
		command = `dpkg --search '*.desktop' | awk '{print $1}' | cut -f 1 -d ':'  | sort --unique`
	case "opensuse-leap":
		command = `zypper --quiet search -i -f --provides "*.desktop" | awk '{print $3}' | sort --unique`
	case "fedora", "almalinux":
		command = `dnf repoquery --installed --file "*.desktop" | awk '{print $1}' | sort --unique`
	case "manjaro", "arch":
		command = `pacman -Ql | grep ".*\.desktop$" | awk '{print $1}' | sort --unique`
	default:
		return errors.New("unsupported operating system version")
	}

	out, err := exec.Command("bash", "-c", command).Output()
	if err != nil {
		return err
	}

	for p := range strings.SplitSeq(string(out), "\n") {
		if p != "" && strings.TrimSpace(p) != "Name" {
			app := openuem_nats.Application{}
			switch os {
			case "debian", "ubuntu", "linuxmint":
				app.Name, app.Version, app.Publisher = getDpkgInfo(p)
			case "fedora", "opensuse-leap", "almalinux":
				app.Name, app.Version, app.Publisher = getRPMInfo(p)
			case "manjaro", "arch":
				app.Name, app.Version, app.Publisher = getPackmanInfo(p)
			}

			// TODO LINUX app.InstallDate
			r.Applications = append(r.Applications, app)
		}
	}

	// Now let's get flatpak apps
	flatpakCommand := `flatpak list | grep system | awk -F'\t' '{print $1 "***" $3}'`
	out, err = exec.Command("bash", "-c", flatpakCommand).Output()
	if err != nil {
		log.Println("[INFO]: could not get apps installed with flatpak")
	} else {
		for p := range strings.SplitSeq(string(out), "\n") {
			if p != "" {
				app := openuem_nats.Application{}
				data := strings.Split(p, "***")
				app.Name = strings.TrimSpace(data[0])
				if len(data) > 1 {
					app.Version = strings.TrimSpace(data[1])
				} else {
					app.Version = "-"
				}
				app.Publisher = "Flatpak"

				r.Applications = append(r.Applications, app)
			}
		}
	}

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

func getDpkgInfo(packageName string) (name string, version string, publisher string) {
	name = ""
	version = ""
	publisher = ""

	out, err := exec.Command("dpkg", "-s", packageName).Output()
	if err != nil {
		return name, version, publisher
	}

	reg := regexp.MustCompile(`Package: \s*(.*?)\s`)
	matches := reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		name = v[1]
		break
	}

	reg = regexp.MustCompile(`Version: \s*(.*?)\s`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		version = v[1]
		break
	}

	reg = regexp.MustCompile(`Original-Maintainer: \s*(.*?)\s<`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		publisher = v[1]
		break
	}

	if publisher == "" {
		reg = regexp.MustCompile(`Vendor: \s*(.*?)\s<`)
		matches = reg.FindAllStringSubmatch(string(out), -1)
		for _, v := range matches {
			publisher = v[1]
			break
		}
	}

	if publisher == "" {
		reg = regexp.MustCompile(`Maintainer: \s*(.*?)\s<`)
		matches = reg.FindAllStringSubmatch(string(out), -1)
		for _, v := range matches {
			publisher = v[1]
			break
		}
	}

	return name, version, publisher
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
