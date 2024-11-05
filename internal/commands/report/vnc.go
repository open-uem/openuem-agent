package report

import (
	"log"

	"github.com/doncicuto/openuem-agent/internal/commands/vnc"
)

func (r *Report) getVNCInfo() error {
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
