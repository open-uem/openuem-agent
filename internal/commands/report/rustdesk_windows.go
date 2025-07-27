//go:build windows

package report

import (
	"log"
	"os"
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
