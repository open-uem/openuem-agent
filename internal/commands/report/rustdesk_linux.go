//go:build linux

package report

import (
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func (r *Report) hasRustDesk(debug bool) {

	if debug {
		log.Println("[DEBUG]: check if RustDesk is available has been requested")
	}

	commonPath := "/usr/bin/rustdesk"
	if _, err := os.Stat(commonPath); err == nil {
		r.HasRustDesk = true
	} else {
		flatpakOpenUEMPath := "/var/lib/flatpak/exports/bin/com.rustdesk.RustDesk"
		if _, err := os.Stat(flatpakOpenUEMPath); err == nil {
			r.HasRustDesk = true
		} else {
			// Get current user logged in
			username, err := runtime.GetLoggedInUser()
			if err == nil {
				// Get home
				u, err := user.Lookup(username)
				if err == nil {
					flatpakUserPath := filepath.Join(u.HomeDir, "exports", "bin", "com.rustdesk.RustDesk")
					if _, err := os.Stat(flatpakUserPath); err == nil {
						r.HasRustDesk = true
					}
				}
			}
		}
	}

	if r.HasRustDesk {
		log.Println("[INFO]: RustDesk is available")
	} else {
		log.Println("[INFO]: RustDesk is not available")
	}
}

func (r *Report) hasRustDeskService(debug bool) {
	r.HasRustDeskService = false
}

func (r *Report) isFlatpakRustDesk() {
	flatpakOpenUEMPath := "/var/lib/flatpak/exports/bin/com.rustdesk.RustDesk"
	if _, err := os.Stat(flatpakOpenUEMPath); err == nil {
		r.IsFlatpakRustDesk = true
		return
	}
}
