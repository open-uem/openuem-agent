//go:build linux

package report

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/open-uem/nats"
)

func (r *Report) getSharesInfo(debug bool) error {
	r.getExportedNFSShares()

	return nil
}

func (r *Report) getExportedNFSShares() {
	shares := []nats.Share{}

	// Use showmount to check if this machine exports NFS shares
	listExportedNFS := `showmount -e --no-headers | awk '{print $1}'`
	out, err := exec.Command("bash", "-c", listExportedNFS).Output()
	if err != nil {
		log.Printf("[ERROR]: could not run the showmount command, reason: %v", err)
		r.Shares = shares
		return
	}

	for s := range strings.SplitSeq(string(out), "\n") {
		if s != "" {
			exportedNFS := nats.Share{}
			exportedNFS.Name = strings.TrimSpace(s)
			exportedNFS.Path = exportedNFS.Name

			// Get information about each share from /etc/exports
			nfsInfo := fmt.Sprintf("cat /etc/exports | grep %s | awk '{print $2}'", exportedNFS.Name)
			out, err = exec.Command("bash", "-c", nfsInfo).Output()
			if err != nil {
				log.Printf("[ERROR]: could not get information from /etc/exports, reason: %v", err)
				continue
			}

			exportedNFS.Description = "NFS :" + strings.TrimSpace(string(out))
			shares = append(shares, exportedNFS)
		}
	}

	r.Shares = shares
}
