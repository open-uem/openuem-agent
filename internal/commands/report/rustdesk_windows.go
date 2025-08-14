//go:build windows

package report

import (
	"log"
	"os"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

func (r *Report) hasRustDesk(debug bool) {
	if debug {
		log.Println("[DEBUG]: check if RustDesk is available has been requested")
	}

	binPath := "C:\\Program Files\\RustDesk\\rustdesk.exe"

	if _, err := os.Stat(binPath); err == nil {
		r.HasRustDesk = true
	}

	if r.HasRustDesk {
		log.Println("[INFO]: RustDesk is available")
	} else {
		log.Println("[INFO]: RustDesk is not available")
	}
}

func (r *Report) hasRustDeskService(debug bool) {

	// check if process rustdesk.exe is running
	psList, err := process.Processes()
	if err != nil {
		log.Printf("[ERROR]: could not get a list of processes running in the machine, reason: %v", err)
		return
	}

	for _, p := range psList {
		name, err := p.Name()
		if err != nil {
			log.Printf("[ERROR]: could not get process name, reason: %v", err)
			break
		}
		if strings.Contains(name, "rustdesk") {
			log.Println("[INFO]: RustDesk is running as a service")
			r.HasRustDeskService = true
			break
		}
	}

	r.HasRustDesk = false
}
