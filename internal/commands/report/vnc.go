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
		log.Println("[INFO]: could not find a supported VNC service")
	} else {
		log.Printf("[INFO]: supported Remote Desktop service found: %s", rd)
	}

	r.SupportedVNCServer = rd
	r.IsWayland = remotedesktop.IsWaylandDisplayServer()
	return nil
}
