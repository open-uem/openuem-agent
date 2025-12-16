//go:build linux

package report

import (
	"log"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/zcalusic/sysinfo"
)

func (r *Report) getOperatingSystemInfo(debug bool) error {
	var si sysinfo.SysInfo

	si.GetSysInfo()

	if debug {
		log.Println("[DEBUG]: operating system info has been requested")
	}

	if si.OS.Release == "" {
		r.OperatingSystem.Version = si.OS.Vendor + " " + si.OS.Version
	} else {
		r.OperatingSystem.Version = si.OS.Vendor + " " + si.OS.Release
	}

	if r.OperatingSystem.Version == "" {
		r.OperatingSystem.Version = "Undetected"
	}

	r.OperatingSystem.Description = si.OS.Name

	r.OperatingSystem.Edition = si.OS.Release
	if r.OperatingSystem.Edition == "" {
		r.OperatingSystem.Edition = "Undetected"
	}

	r.OperatingSystem.Arch = si.OS.Architecture
	if r.OperatingSystem.Arch == "" {
		r.OperatingSystem.Arch = "Undetected"
	}

	if err := r.getOSInstallationDate(); err != nil {
		log.Printf("[ERROR]: could not get OS installation date: %v", err)
		return err
	} else {
		log.Printf("[INFO]: OS installation date has been retrieved from Linux")
	}

	if err := r.getSysBootupTime(); err != nil {
		log.Printf("[ERROR]: could not get system boot up time: %v", err)
		return err
	} else {
		log.Printf("[INFO]: system boot up time has been retrieved from Linux")
	}

	if debug {
		log.Println("[DEBUG]: username info has been requested")
	}
	if err := r.getUsername(); err != nil {
		log.Printf("[ERROR]: could not get current username from Linux: %v", err)
		return err
	} else {
		log.Printf("[INFO]: linux username has been retrieved from Linux")
	}
	return nil
}

func (r *Report) getUsername() error {
	cmd := "who | grep -m1 seat0 | awk '{print $1}'"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	r.OperatingSystem.Username = strings.TrimSpace(string(out))

	return nil
}

func (r *Report) getOSInstallationDate() error {
	// Ref: https://unix.stackexchange.com/questions/9971/how-do-i-find-how-long-ago-a-linux-system-was-installed
	cmd := "ls -alct --full-time /|tail -1|awk '{print $6}'"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return err
	}

	installationDate := strings.TrimSpace(string(out))

	// try to get /etc/machine-id date
	if installationDate == "1970-01-01" {
		cmd := "ls -alct --full-time /etc/machine-id |tail -1|awk '{print $6}'"
		out, err = exec.Command("bash", "-c", cmd).Output()
		if err != nil {
			return err
		}
		installationDate = strings.TrimSpace(string(out))
	}

	t, err := time.Parse("2006-01-02", installationDate)
	if err != nil {
		return err
	}

	r.OperatingSystem.InstallDate = t
	return nil
}

func (r *Report) getSysBootupTime() error {
	in := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(in)
	if err != nil {
		return err
	}

	now := time.Now()
	r.OperatingSystem.LastBootUpTime = now.Add(time.Duration(-1*in.Uptime) * time.Second)

	return nil
}
