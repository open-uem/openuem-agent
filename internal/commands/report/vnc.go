package report

import (
	"log"

	"github.com/doncicuto/openuem-agent/internal/commands/vnc"
)

func (r *Report) getVNCInfo() {
	v, err := vnc.GetSupportedVNCServer("")
	if err != nil {
		log.Println("[ERROR]: could not find a supported VNC server")
	}

	r.SupportedVNCServer = v.Name
}
