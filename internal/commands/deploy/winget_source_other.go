//go:build !windows

package deploy

import "errors"

// RegisterWinGetSource is only supported on Windows
func RegisterWinGetSource(name, sourceURL, caCertPath string, trustCA bool) error {
	return errors.New("winget sources are only supported on Windows")
}
