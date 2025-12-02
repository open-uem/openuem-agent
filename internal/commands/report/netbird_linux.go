//go:build linux

package report

import (
	"os"
	"os/exec"
	"strings"
)

func (r *Report) getNetbirdInfo() error {
	netbirdBin := "/usr/bin/netbird"

	_, err := os.Stat(netbirdBin)
	if err == nil {
		r.Netbird.Installed = true
		out, err := exec.Command(netbirdBin, "version").CombinedOutput()
		if err == nil {
			r.Netbird.Version = strings.TrimSpace(string(out))
		}
	}

	return nil
}
