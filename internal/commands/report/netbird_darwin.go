//go:build darwin

package report

import (
	"os/exec"
	"strings"

	"github.com/open-uem/nats"
)

func (r *Report) getNetbirdInfo() error {
	out, err := exec.Command("which", "netbird").Output()
	if err == nil {
		netbirdBin := strings.TrimSpace(string(out))
		r.Netbird.Installed = true
		out, err := exec.Command(netbirdBin, "version").CombinedOutput()
		if err == nil {
			r.Netbird.Version = strings.TrimSpace(string(out))
		}
	} else {

	}

	return nil
}

func RetrieveNetbirdInfo() (*nats.Netbird, error) {
	return nil, nil
}
