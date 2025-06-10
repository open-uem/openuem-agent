//go:build darwin

package report

import (
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/open-uem/nats"
	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getSystemUpdateInfo() error {
	r.CheckUpdatesStatus()
	r.CheckSecurityUpdatesAvailable()
	r.CheckSecurityUpdatesLastSearch()
	r.getUpdatesHistory()
	return nil
}

func (r *Report) CheckSecurityUpdatesAvailable() {
	out, err := exec.Command("softwareupdate", "-l").Output()

	if err != nil {
		log.Printf("[ERROR]: could not run softwareupdate -l, reason: %v", err)
		return
	}

	r.SystemUpdate.PendingUpdates = !strings.Contains(string(out), "No new software available")
}

func (r *Report) CheckUpdatesStatus() {
	var download, automatic bool
	automaticDownloadsCmd := `defaults read /Library/Preferences/com.apple.SoftwareUpdate.plist AutomaticDownload`
	out, err := exec.Command("bash", "-c", automaticDownloadsCmd).Output()
	if err != nil {
		log.Printf("[ERROR]: could not read SoftwareUpdate.plist, reason: %v", err)
		download = true
	} else {
		downloadsOut := strings.TrimSpace(string(out))
		download, err = strconv.ParseBool(downloadsOut)
		if err != nil {
			r.SystemUpdate.Status = nats.NOT_CONFIGURED
			return
		}
	}

	if !download {
		r.SystemUpdate.Status = nats.NOTIFY_BEFORE_DOWNLOAD
		return
	}

	automaticInstallCmd := `defaults read /Library/Preferences/com.apple.SoftwareUpdate.plist AutomaticallyInstallMacOSUpdates`
	out, err = exec.Command("bash", "-c", automaticInstallCmd).Output()
	if err != nil {
		log.Printf("[ERROR]: could not read SoftwareUpdate.plist, reason: %v", err)
		r.SystemUpdate.Status = nats.NOTIFY_BEFORE_INSTALLATION
		automatic = false
	} else {
		automaticOut := strings.TrimSpace(string(out))
		automatic, err = strconv.ParseBool(automaticOut)
		if err != nil {
			r.SystemUpdate.Status = nats.NOTIFY_BEFORE_INSTALLATION
			return
		}
	}

	if automatic {
		r.SystemUpdate.Status = nats.NOTIFY_SCHEDULED_INSTALLATION
		return
	} else {
		r.SystemUpdate.Status = nats.NOTIFY_BEFORE_INSTALLATION
		return
	}
}

func (r *Report) CheckSecurityUpdatesLastSearch() {
	lastSearchCmd := `defaults read /Library/Preferences/com.apple.SoftwareUpdate.plist LastSuccessfulDate`
	out, err := exec.Command("bash", "-c", lastSearchCmd).Output()
	if err != nil {
		log.Printf("[ERROR]: could not read SoftwareUpdate.plist, reason: %v", err)
		return
	}

	//2025-06-04 12:05:57 +0000
	lastSearch, err := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(string(out)))
	if err != nil {
		log.Printf("[ERROR]: could not parse date from SoftwareUpdate.plist, reason: %v", err)
		return
	}

	r.SystemUpdate.LastSearch = lastSearch
}

func (r *Report) getUpdatesHistory() error {
	listUpdatesCmd := `softwareupdate --history | grep -v Display | grep -v '\-'`
	out, err := exec.Command("bash", "-c", listUpdatesCmd).Output()
	if err != nil {
		log.Printf("[ERROR]: could not read software update history, reason: %v", err)
		return err
	}

	lines := strings.Split(string(out), "\n")

	updates := []openuem_nats.Update{}
	for _, entry := range lines {
		update := openuem_nats.Update{
			Title: strings.Join(strings.Fields(entry), " "),
		}
		updates = append(updates, update)
	}
	r.Updates = updates

	return nil
}
