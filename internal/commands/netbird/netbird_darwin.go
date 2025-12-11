//go:build darwin

package netbird

import (
	"log"
	"os"
	"os/exec"
	"strings"

	openuem_nats "github.com/open-uem/nats"
)

func Install() (*openuem_nats.Netbird, error) {
	var err error

	c1 := exec.Command("curl", "-fsSL", "https://pkgs.netbird.io/install.sh")
	c2 := exec.Command("sh")
	c2.Stdin, err = c1.StdoutPipe()
	if err != nil {
		log.Printf("[ERROR]: could not create the pipe for the curl install NetBird command, reason: %v", err)
		return nil, err
	}
	c2.Stdout = os.Stdout
	if err := c2.Start(); err != nil {
		log.Printf("[ERROR]: could not start the sh command for NetBird install command, reason: %v", err)
		return nil, err
	}
	if err := c1.Run(); err != nil {
		log.Printf("[ERROR]: could not download the NetBird install script, reason: %v", err)
		return nil, err
	}
	if err := c2.Wait(); err != nil {
		log.Printf("[ERROR]: could not install the NetBird client, reason: %v", err)
		return nil, err
	}
	return nil, nil
}

func Uninstall() error {
	var err error

	c1 := exec.Command("curl", "-fsSL", "https://downloads.openuem.eu/netbird/netbird_uninstall.sh")
	c2 := exec.Command("sh")
	c2.Stdin, err = c1.StdoutPipe()
	if err != nil {
		log.Printf("[ERROR]: could not create the pipe for the curl uninstall NetBird script, reason: %v", err)
		return err
	}
	c2.Stdout = os.Stdout
	if err := c2.Start(); err != nil {
		log.Printf("[ERROR]: could not start the sh command for NetBird uninstall script, reason: %v", err)
		return err
	}
	if err := c1.Run(); err != nil {
		log.Printf("[ERROR]: could not download the NetBird uninstall script, reason: %v", err)
		return err
	}
	if err := c2.Wait(); err != nil {
		log.Printf("[ERROR]: could not uninstall the NetBird client, reason: %v", err)
		return err
	}
	return nil
}

func SwitchProfile(data []byte) (*openuem_nats.Netbird, error) {
	return nil, nil
}

func RefreshInfo(data []byte) (*openuem_nats.Netbird, error) {
	return nil, nil
}

func getNetbirdBin() string {
	netbirdBin := "/usr/local/bin/netbird"
	out, err := exec.Command("which", "netbird").Output()
	if err == nil {
		netbirdBin = strings.TrimSpace(string(out))
	}

	return netbirdBin
}
