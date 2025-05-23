package report

import (
	"log"

	remotedesktop "github.com/open-uem/openuem-agent/internal/commands/remote-desktop"
)

func (r *Report) getRemoteDesktopInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: remote desktop info has been requested")
	}

	rd := remotedesktop.GetSupportedRemoteDesktop(r.OS)
	if rd == "" {
		log.Println("[ERROR]: could not find a supported Remote Desktop service")
	} else {
		log.Printf("[INFO]: supported Remote Desktop service found: %s", rd)
	}

	r.SupportedVNCServer = rd
	return nil
}
