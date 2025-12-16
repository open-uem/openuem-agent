//go:build windows

package netbird

import (
	"encoding/json"
	"log"
	"os/exec"
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

func SwitchProfile(request openuem_nats.NetbirdSwitchProfile) (*openuem_nats.Netbird, error) {
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

func Register(data []byte) (*openuem_nats.Netbird, error) {
	request := openuem_nats.NetbirdRegister{}
	if err := json.Unmarshal(data, &request); err != nil {
		log.Printf("[ERROR]: could not unmarshal the NetBird register request, reason: %v", err)
		return nil, err
	}

	bin := getNetbirdBin()

	// First, we must set the connection down
	if err := exec.Command(bin, "down").Run(); err != nil {
		log.Println("[ERROR]: could not execute netbird down")
		return nil, err
	}

	// Now, use the key and URL to register the agent
	if err := exec.Command(bin, "up", "--setup-key", request.OneOffKey, "--management-url", request.ManagementURL).Run(); err != nil {
		log.Println("[ERROR]: could not execute netbird up")
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
