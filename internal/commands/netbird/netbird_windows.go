//go:build windows

package netbird

import (
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/deploy"
)

func Install() (*openuem_nats.Netbird, error) {
	return nil, deploy.InstallPackage("Netbird.Netbird", "", false, false)
}

func Uninstall() error {
	return deploy.UninstallPackage("Netbird.Netbird")
}

func SwitchProfile(data []byte) (*openuem_nats.Netbird, error) {
	return nil, nil
}

func getNetbirdBin() string {
	return "C:\\Program Files\\NetBird"
}
