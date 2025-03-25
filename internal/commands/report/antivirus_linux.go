//go:build linux

package report

import (
	"errors"
)

// TODO LINUX
func (r *Report) getAntivirusInfo(debug bool) error {
	return errors.New("not implemented in Linux, yet")
}
