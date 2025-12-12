//go:build windows

package netbird

import (
	"encoding/json"
	"log"
	"time"

	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/deploy"
	"github.com/open-uem/openuem-agent/internal/commands/report"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func Install() (*openuem_nats.Netbird, error) {
	if err := deploy.InstallPackage("Netbird.Netbird", "", false, false); err != nil {
		log.Printf("[ERROR]: could not install the NetBird client, reason: %v", err)
		return nil, err
	}

	return report.RetrieveNetbirdInfo()
}

func Uninstall() error {
	return deploy.UninstallPackage("Netbird.Netbird")
}

func SwitchProfile(data []byte) (*openuem_nats.Netbird, error) {
	request := openuem_nats.NetbirdSwitchProfile{}
	if err := json.Unmarshal(data, &request); err != nil {
		log.Printf("[ERROR]: could not unmarshal the NetBird switch profile request, reason: %v", err)
		return nil, err
	}

	netBirdBin := getNetbirdBin()

	args := []string{"profile", "select", request.Profile}
	out, err := runtime.RunAsUserWithOutput(netBirdBin, args)
	if err != nil {
		log.Printf("[ERROR]: could not switch NetBird profile, reason: %s", string(out))
		return nil, err
	}

	args = []string{"up"}
	out, err = runtime.RunAsUserWithOutputAndTimeout(netBirdBin, args, 60*time.Second)
	if err != nil {
		log.Printf("[ERROR]: could not switch NetBird profile, reason: %s", string(out))
		return nil, err
	}

	return report.RetrieveNetbirdInfo()
}

func NetbirdUp(data []byte) (*openuem_nats.Netbird, error) {
	netBirdBin := getNetbirdBin()

	args := []string{"up"}
	out, err := runtime.RunAsUserWithOutputAndTimeout(netBirdBin, args, 60*time.Second)
	if err != nil {
		log.Printf("[ERROR]: could not execute netbird up, reason: %s", string(out))
		return nil, err
	}

	return report.RetrieveNetbirdInfo()
}

func NetbirdDown(data []byte) (*openuem_nats.Netbird, error) {
	netBirdBin := getNetbirdBin()

	args := []string{"down"}
	out, err := runtime.RunAsUserWithOutputAndTimeout(netBirdBin, args, 60*time.Second)
	if err != nil {
		log.Printf("[ERROR]: could not execute netbird down, reason: %s", string(out))
		return nil, err
	}

	return report.RetrieveNetbirdInfo()
}

func getNetbirdBin() string {
	return "C:\\Program Files\\NetBird\\netbird.exe"
}
