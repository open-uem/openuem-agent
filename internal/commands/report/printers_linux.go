//go:build linux

package report

import "errors"

func (r *Report) getPrintersInfo(debug bool) error {
	return errors.New("not implemented in Linux, yet")
}
