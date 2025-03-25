//go:build linux

package report

import (
	"errors"

	openuem_nats "github.com/open-uem/nats"
)

// TODO LINUX - Get applications installed
func getApplications(debug bool) (map[string]openuem_nats.Application, error) {
	return nil, errors.New("not implemented in Linux, yet")
}
