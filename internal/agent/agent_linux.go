//go:build linux

package agent

import "errors"

func (a *Agent) StartVNCSubscribe() error {
	return errors.New("not implemented in Linux, yet")
}
