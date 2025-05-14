//go:build linux

package report

import (
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/open-uem/nats"
)

func (r *Report) getSystemUpdateInfo() error {
	switch r.OS {
	case "ubuntu", "debian", "linuxmint":
		if err := r.getAptInformation(); err != nil {
			log.Printf("[ERROR]: could not get pending security updates, reason: %v", err)
		} else {
			log.Println("[INFO]: get pending security updates info has been retrieved")
		}
	case "fedora", "almalinux", "redhat", "rocky":
		if err := r.getDnfInformation(); err != nil {
			log.Printf("[ERROR]: could not get pending security updates, reason: %v", err)
		} else {
			log.Println("[INFO]: get pending security updates info has been retrieved")
		}
	}

	return nil
}

func (r *Report) getAptInformation() error {

	// Check if we've security updates that can be upgraded
	r.SystemUpdate.PendingUpdates = checkAptSecurityUpdatesAvailable()

	// Check if unattended is running
	r.SystemUpdate.Status = checkUpdatesStatus()

	// Check last time packages were installed
	r.SystemUpdate.LastInstall = checkLastTimePackagesInstalled()

	return nil
}

func (r *Report) getDnfInformation() error {

	// Check if we've security updates that can be upgraded
	r.SystemUpdate.PendingUpdates = checkDnfSecurityUpdatesAvailable()

	// Check if unattended is running
	r.SystemUpdate.Status = checkDnfUpdatesStatus()

	// Check last time packages were installed
	r.SystemUpdate.LastInstall = checkDnfLastTimePackagesInstalled()

	return nil
}

func checkAptSecurityUpdatesAvailable() bool {
	if err := exec.Command("apt", "update").Run(); err != nil {
		log.Printf("[ERROR]: could not run apt update, reason: %v", err)
		return false
	}

	secUpdatesAvailable := `apt list --upgradable 2>/dev/null | grep "\-security" | wc -l`
	out, err := exec.Command("bash", "-c", secUpdatesAvailable).Output()
	if err != nil {
		log.Printf("[ERROR]: could not check if updates are available, reason: %v", err)
		return false
	}

	nUpdates, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		log.Printf("[ERROR]: could not get the number of updates available, reason: %v", err)
		return false
	}

	return nUpdates > 0
}

func checkDnfSecurityUpdatesAvailable() bool {
	secUpdatesAvailable := `dnf check-update --refresh --security | wc -l`
	out, err := exec.Command("bash", "-c", secUpdatesAvailable).Output()
	if err != nil {
		log.Printf("[ERROR]: could not check if updates are available, reason: %v", err)
		return false
	}

	nUpdates, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		log.Printf("[ERROR]: could not get the number of updates available, reason: %v", err)
		return false
	}

	return nUpdates > 0
}

func checkUpdatesStatus() string {

	unattendedCheck := `grep unattended /var/log/apt/history.log | wc -l`
	out, err := exec.Command("bash", "-c", unattendedCheck).Output()
	if err != nil {
		log.Printf("[ERROR]: could not read APT history log, reason: %v", err)
		return nats.NOT_CONFIGURED
	}

	nUnattended, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		log.Printf("[ERROR]: could not get the number of unattended items found in APT history log, reason: %v", err)
		return nats.NOT_CONFIGURED
	}

	if nUnattended > 0 {
		return nats.NOTIFY_SCHEDULED_INSTALLATION
	} else {
		return nats.NOT_CONFIGURED
	}
}

func checkLastTimePackagesInstalled() time.Time {
	lastInstall := `dnf history list | grep upgrade | awk '{print $5,$6}'`
	out, err := exec.Command("bash", "-c", lastInstall).Output()
	if err != nil {
		log.Printf("[ERROR]: could not read DNF history log, reason: %v", err)
		return time.Time{}
	}

	t, err := time.Parse("2006-01-02 15:04:05", strings.TrimSpace(string(out)))
	if err != nil {
		log.Printf("[ERROR]: could not parse time string %s from APT history log, reason: %v", string(out), err)
		return time.Time{}
	}

	return t
}

func checkDnfLastTimePackagesInstalled() time.Time {
	lastInstall := `tail -3 /var/log/apt/history.log | grep End-Date | awk '{print $2,$3}'`
	out, err := exec.Command("bash", "-c", lastInstall).Output()
	if err != nil {
		log.Printf("[ERROR]: could not read APT history log, reason: %v", err)
		return time.Time{}
	}

	t, err := time.Parse("2006-01-02 15:04:05", strings.TrimSpace(string(out)))
	if err != nil {
		log.Printf("[ERROR]: could not parse time string %s from APT history log, reason: %v", string(out), err)
		return time.Time{}
	}

	return t
}

func checkDnfUpdatesStatus() string {
	_, err := os.Stat("/etc/dnf/automatic.conf")
	if err == nil {
		return nats.NOTIFY_SCHEDULED_INSTALLATION
	} else {
		return nats.NOT_CONFIGURED
	}
}
