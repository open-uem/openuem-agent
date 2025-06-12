//go:build darwin

package report

import (
	"log"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func (r *Report) getOperatingSystemInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: operating system info has been requested")
	}

	if err := r.getOSInfo(); err != nil {
		log.Printf("[ERROR]: could not get OS info: %v", err)
		return err
	} else {
		log.Printf("[INFO]: OS info has been retrieved")
	}

	if err := r.getOSInstallationDate(); err != nil {
		log.Printf("[ERROR]: could not get OS installation date: %v", err)
		return err
	} else {
		log.Printf("[INFO]: OS installation date has been retrieved")
	}

	if err := r.getSysBootupTime(); err != nil {
		log.Printf("[ERROR]: could not get system boot up time: %v", err)
		return err
	} else {
		log.Printf("[INFO]: system boot up time has been retrieved")
	}

	if debug {
		log.Println("[DEBUG]: username info has been requested")
	}
	if err := r.getUsername(); err != nil {
		log.Printf("[ERROR]: could not get current username: %v", err)
		return err
	} else {
		log.Printf("[INFO]: username has been retrieved")
	}
	return nil
}

func (r *Report) getOSInfo() error {
	cmd := "sw_vers --ProductName"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}
	name := strings.TrimSpace(string(out))

	cmd = "sw_vers --ProductVersion"
	out, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(out))

	cmd = "sw_vers --BuildVersion"
	out, err = exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}
	buildVersion := strings.TrimSpace(string(out))

	r.OperatingSystem.Version = name + " " + version
	r.OperatingSystem.Description = r.OperatingSystem.Version + " " + getMacOSName(version) + " (" + buildVersion + ")"
	r.OperatingSystem.Edition = buildVersion
	r.OperatingSystem.Arch = getMacOSArch()

	return nil
}

func (r *Report) getUsername() error {
	cmd := "stat -f '%Su' /dev/console"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	r.OperatingSystem.Username = strings.TrimSpace(string(out))

	return nil
}

func (r *Report) getOSInstallationDate() error {

	cmd := `stat -f "%SB" /var/db/.AppleSetupDone`
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	installationDate := strings.TrimSpace(string(out))
	t, err := time.Parse("Jan 2 15:04:05 2006", installationDate)
	if err != nil {
		return err
	}

	r.OperatingSystem.InstallDate = t
	return nil
}

func getMacOSArch() string {
	cmd := "uname -m"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return runtime.GOARCH
	}
	return string(out)
}

func (r *Report) getSysBootupTime() error {
	cmd := `sysctl kern.boottime | awk '{ print $11,$12,$13,$14}'`
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	sysBootUpTime := strings.TrimSpace(string(out))
	t, err := time.Parse("Jan 2 15:04:05 2006", sysBootUpTime)
	if err != nil {
		return err
	}

	r.OperatingSystem.LastBootUpTime = t
	return nil
}

func getMacOSName(version string) string {
	versionNumbers := strings.Split(version, ".")

	if len(versionNumbers) == 0 {
		return ""
	}

	switch versionNumbers[0] {
	case "15":
		return "Sequoia"
	case "14":
		return "Sonoma"
	case "13":
		return "Ventura"
	case "12":
		return "Monterey"
	case "11":
		return "Big Sur"
	case "10":
		if len(versionNumbers) < 2 {
			return ""
		}
		switch versionNumbers[1] {
		case "15":
			return "Catalina"
		case "14":
			return "Mojave"
		case "13":
			return "High Sierra"
		case "12":
			return "Sierra"
		case "11":
			return "El Capitan"
		case "10":
			return "Yosemite"
		}
	}

	return ""
}
