package report

import (
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func (r *Report) hasRustDesk(debug bool) {

	installed := false

	if debug {
		log.Println("[DEBUG]: check if RustDesk is available has been requested")
	}

	commonPath := "/usr/bin/rustdesk"
	if _, err := os.Stat(commonPath); err == nil {
		installed = true
	} else {
		flatpakOpenUEMPath := "/var/lib/flatpak/exports/bin/com.rustdesk.RustDesk"
		if _, err := os.Stat(flatpakOpenUEMPath); err == nil {
			installed = true
		} else {
			// Get current user logged in
			username, err := runtime.GetLoggedInUser()
			if err == nil {
				// Get home
				u, err := user.Lookup(username)
				if err == nil {
					flatpakUserPath := filepath.Join(u.HomeDir, "exports", "bin", "com.rustdesk.RustDesk")
					if _, err := os.Stat(flatpakUserPath); err == nil {
						installed = true
					}
				}
			}
		}
	}

	r.HasRustDesk = installed

	if installed {
		log.Println("[INFO]: RustDesk is available")
	} else {
		log.Println("[INFO]: RustDesk is not available")
	}
}
