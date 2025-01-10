package report

import (
	"log"

	"github.com/open-uem/openuem-agent/internal/commands/vnc"
)

func (r *Report) getVNCInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: vnc info has been requested")
	}

	v, err := vnc.GetSupportedVNCServer("")
	if err != nil {
		log.Println("[ERROR]: could not find a supported VNC server")
		r.SupportedVNCServer = ""
		return err
	} else {
		r.SupportedVNCServer = v.Name
		log.Printf("[INFO]: supported VNC server found: %s", v.Name)
	}
	return nil
}
