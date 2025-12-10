//go:build windows

package report

import (
	"os"
	"os/exec"
	"strings"

	"github.com/open-uem/nats"
)

func (r *Report) getNetbirdInfo() error {
	netbirdBin := "C:\\Program Files\\NetBird"

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

func RetrieveNetbirdInfo() (*nats.Netbird, error) {
	return nil, nil
}
