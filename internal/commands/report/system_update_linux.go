//go:build linux

package report

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func (r *Report) getSystemUpdateInfo() error {
	switch r.OS {
	case "ubuntu", "debian", "linuxmint", "neon":
		if err := r.getAptInformation(); err != nil {
			log.Printf("[ERROR]: could not get pending security updates, reason: %v", err)
		} else {
			log.Println("[INFO]: get pending security updates info has been retrieved")
		}
	case "fedora", "almalinux", "redhat", "rocky":
		if strings.Contains(r.OperatingSystem.Description, "Silverblue") || strings.Contains(r.OperatingSystem.Description, "Kinoite") {
			if err := r.getSilverblueInformation(); err != nil {
				log.Printf("[ERROR]: could not get pending security updates, reason: %v", err)
			} else {
				log.Println("[INFO]: get pending security updates info has been retrieved")
			}
		} else {
			if err := r.getDnfInformation(); err != nil {
				log.Printf("[ERROR]: could not get pending security updates, reason: %v", err)
			} else {
				log.Println("[INFO]: get pending security updates info has been retrieved")
			}
		}

	default:
		r.SystemUpdate.Status = nats.UNKNOWN
	}

	return nil
}

func (r *Report) getAptInformation() error {

	// Check if we've security updates that can be upgraded
	r.SystemUpdate.PendingUpdates = checkAptSecurityUpdatesAvailable()

	// Check if unattended is running
	r.SystemUpdate.Status = checkUpdatesStatus()

	// Check if gnome software updates is set
	if r.SystemUpdate.Status == nats.NOT_CONFIGURED && IsGnomeDesktop() && IsGnomeSoftwareUpdatesEnabled() {
		r.SystemUpdate.Status = nats.NOTIFY_SCHEDULED_INSTALLATION
	}

	// Check if KDE software updates is set
	if r.SystemUpdate.Status == nats.NOT_CONFIGURED && IsKDEDesktop() && IsKDESoftwareUpdatesEnabled() {
		r.SystemUpdate.Status = nats.NOTIFY_SCHEDULED_INSTALLATION
	}

	// Check last time packages were installed
	r.SystemUpdate.LastInstall = checkLastTimePackagesInstalled()

	return nil
}

func (r *Report) getDnfInformation() error {

	// Check if we've security updates that can be upgraded
	r.SystemUpdate.PendingUpdates = checkDnfSecurityUpdatesAvailable()

	// Check if unattended is running
	r.SystemUpdate.Status = checkDnfUpdatesStatus()

	// Check if gnome software updares is set
	if r.SystemUpdate.Status == nats.NOT_CONFIGURED && IsGnomeDesktop() && IsGnomeSoftwareUpdatesEnabled() {
		r.SystemUpdate.Status = nats.NOTIFY_SCHEDULED_INSTALLATION
	}

	// Check if KDE software updates is set
	if r.SystemUpdate.Status == nats.NOT_CONFIGURED && IsKDEDesktop() && IsKDESoftwareUpdatesEnabled() {
		r.SystemUpdate.Status = nats.NOTIFY_SCHEDULED_INSTALLATION
	}

	// Check last time packages were installed
	r.SystemUpdate.LastInstall = checkDnfLastTimePackagesInstalled()

	return nil
}

func (r *Report) getSilverblueInformation() error {

	// Check if we've security updates that can be upgraded
	r.SystemUpdate.PendingUpdates = checkSilverblueSecurityUpdatesAvailable()

	// Check if gnome software updares is set
	if IsGnomeDesktop() && IsGnomeSoftwareUpdatesEnabled() {
		r.SystemUpdate.Status = nats.NOTIFY_SCHEDULED_INSTALLATION
	}

	// Check if KDE software updates is set
	if IsKDEDesktop() && IsKDESoftwareUpdatesEnabled() {
		r.SystemUpdate.Status = nats.NOTIFY_SCHEDULED_INSTALLATION
	}

	// Check last time packages were installed
	r.SystemUpdate.LastInstall = checkSilverblueLastTimePackagesInstalled()

	return nil
}

func checkAptSecurityUpdatesAvailable() bool {
	if err := exec.Command("apt", "update").Run(); err != nil {
		log.Printf("[ERROR]: could not run apt update, reason: %v", err)
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
	out, err := exec.Command("apt-config", "dump", "APT::Periodic::Unattended-Upgrade").Output()
	if err != nil {
		log.Printf("[ERROR]: could not dump apt-config for Unattended-Upgrade, reason: %v", err)
		return nats.NOT_CONFIGURED
	}

	unattendedUpgrade := strings.TrimSpace(string(out))

	if unattendedUpgrade != "" &&
		strings.Contains(unattendedUpgrade, "APT::Periodic::Unattended-Upgrade") &&
		!strings.Contains(unattendedUpgrade, `APT::Periodic::Unattended-Upgrade "0";`) {
		return nats.NOTIFY_SCHEDULED_INSTALLATION
	}

	return nats.NOT_CONFIGURED
}

func checkLastTimePackagesInstalled() time.Time {
	lastInstall := `grep "Upgrade:" -B 4 /var/log/apt/history.log | grep -v "Upgrade:" | grep Start-Date | tail -1 | awk '{print $2,$3}'`
	out, err := exec.Command("bash", "-c", lastInstall).Output()
	if err != nil {
		log.Printf("[ERROR]: could not read DNF history log, reason: %v", err)
		return time.Time{}
	}

	history := strings.TrimSpace(string(out))
	if history == "" {
		log.Println("[INFO]: no info available from /var/log/apt/history.log")
		return time.Time{}
	}

	loc, err := time.LoadLocation("Local")
	if err != nil {
		return time.Time{}
	}

	t, err := time.ParseInLocation("2006-01-02 15:04:05", history, loc)
	if err != nil {
		log.Printf("[ERROR]: could not parse time string %s from APT history log, reason: %v", string(out), err)
		return time.Time{}
	}

	return t
}

func checkDnfLastTimePackagesInstalled() time.Time {
	var t time.Time

	lastInstall := `dnf history list | grep -m 1 update | awk '{print $5,$6}'`
	out, err := exec.Command("bash", "-c", lastInstall).Output()
	if err != nil {
		log.Printf("[ERROR]: could not read DNF history log, reason: %v", err)
		return time.Time{}
	}

	history := strings.TrimSpace(string(out))
	if history == "" {
		log.Println("[INFO]: no info available from dnf history list")
		return time.Time{}
	}

	loc, err := time.LoadLocation("Local")
	if err != nil {
		return time.Time{}
	}

	if t, err = time.ParseInLocation("2006-01-02 15:04", history, loc); err == nil {
		return t
	}

	if t, err = time.ParseInLocation("2006-01-02 15:04:05", history, loc); err == nil {
		return t
	}

	lastInstall = `dnf history list | grep -m 1 update | awk '{print $6,$7}'`
	out, err = exec.Command("bash", "-c", lastInstall).Output()
	if err != nil {
		log.Printf("[ERROR]: could not read DNF history log, reason: %v", err)
		return time.Time{}
	}

	history = strings.TrimSpace(string(out))
	if history == "" {
		log.Println("[INFO]: no info available from dnf history list")
		return time.Time{}
	}

	if t, err = time.ParseInLocation("2006-01-02 15:04", history, loc); err == nil {
		return t
	}

	if t, err = time.ParseInLocation("2006-01-02 15:04:05", history, loc); err == nil {
		return t
	}

	return time.Time{}
}

func checkDnfUpdatesStatus() string {
	_, err := os.Stat("/etc/dnf/automatic.conf")
	if err == nil {
		conf, err := os.ReadFile("/etc/dnf/automatic.conf")
		if err == nil && strings.Contains(string(conf), "apply_updates=True") {
			return nats.NOTIFY_SCHEDULED_INSTALLATION
		}
	}

	return nats.NOT_CONFIGURED
}

func checkSilverblueSecurityUpdatesAvailable() bool {
	secUpdatesAvailable := `sudo rpm-ostree upgrade --check | grep SecAdvisories | wc -l`
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

func checkSilverblueLastTimePackagesInstalled() time.Time {
	var t time.Time

	lastInstall := `sudo rpm-ostree status | grep -m1 Version | awk '{print $3}'`
	out, err := exec.Command("bash", "-c", lastInstall).Output()
	if err != nil {
		log.Printf("[ERROR]: could not read DNF history log, reason: %v", err)
		return time.Time{}
	}

	history := strings.TrimSpace(string(out))
	if history == "" {
		log.Println("[INFO]: no info available from dnf history list")
		return time.Time{}
	}

	loc, err := time.LoadLocation("Local")
	if err != nil {
		return time.Time{}
	}

	if t, err = time.ParseInLocation("(2006-01-02T15:04:05Z)", history, loc); err == nil {
		return t
	}

	return time.Time{}
}

// func checkZypperLastTimePackagesInstalled() time.Time {
// 	lastInstall := `grep "'update'" /var/log/zypp/history | cut -f 1 -d '|'`
// 	out, err := exec.Command("bash", "-c", lastInstall).Output()
// 	if err != nil {
// 		log.Printf("[ERROR]: could not read DNF history log, reason: %v", err)
// 		return time.Time{}
// 	}

// 	history := strings.TrimSpace(string(out))
// 	if history == "" {
// 		log.Println("[INFO]: no info available from /var/log/zypp/history")
// 	}

// 	t, err := time.Parse("2006-01-02 15:04:05", history)
// 	if err != nil {
// 		log.Printf("[ERROR]: could not parse time string %s from Zypper history log, reason: %v", string(out), err)
// 		return time.Time{}
// 	}

// 	return t
// }

func IsGnomeDesktop() bool {
	session, err := runtime.GetUserEnv("XDG_SESSION_DESKTOP")
	return err == nil && strings.ToLower(session) == "gnome"
}

func IsKDEDesktop() bool {
	session, err := runtime.GetUserEnv("XDG_SESSION_DESKTOP")
	return err == nil && strings.ToLower(session) == "kde"
}

func IsGnomeSoftwareUpdatesEnabled() bool {
	username, err := runtime.GetLoggedInUser()
	if err != nil {
		return false
	}

	args := []string{"read", "/org/gnome/software/download-updates"}
	out, err := runtime.RunAsUserWithOutput(username, "/usr/bin/dconf", args, true)
	if err != nil {
		log.Printf("[INFO]: could not find the dconf entry for download-updates, reason %v", err)
		return false
	}

	dconfOut := strings.TrimSpace(string(out))
	if dconfOut != "" {
		enabled, err := strconv.ParseBool(dconfOut)
		if err != nil {
			return false
		}
		return enabled
	}

	return true
}

func IsKDESoftwareUpdatesEnabled() bool {
	home, err := runtime.GetUserEnv("HOME")
	if err != nil {
		return false
	}

	command := fmt.Sprintf(`grep Unattended %s/.config/PlasmaDiscoverUpdates`, home)
	out, err := exec.Command("bash", "-c", command).Output()
	if err != nil {
		return false
	}

	if strings.TrimSpace(string(out)) != "" {
		return true
	}
	return false
}
