//go:build windows

package netbird

import (
	"github.com/open-uem/openuem-agent/internal/commands/deploy"
)

func Install() error {
	return deploy.InstallPackage("Netbird.Netbird", "", false, false)
}
