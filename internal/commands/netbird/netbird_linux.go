//go:build linux

package netbird

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/open-uem/nats"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/report"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func Install() (*openuem_nats.Netbird, error) {
	var err error

	c1 := exec.Command("curl", "-fsSL", "https://pkgs.netbird.io/install.sh")
	c2 := exec.Command("sh")
	desktop, err := runtime.GetUserEnv("XDG_CURRENT_DESKTOP")
	if err == nil {
		c2.Env = os.Environ()
		c2.Env = append(c2.Env, fmt.Sprintf("XDG_CURRENT_DESKTOP=%s", desktop))
	}
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

	return report.RetrieveNetbirdInfo()
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

func SwitchProfile(data []byte) (*nats.Netbird, error) {
	request := openuem_nats.NetbirdSwitchProfile{}
	if err := json.Unmarshal(data, &request); err != nil {
		log.Printf("[ERROR]: could not unmarshal the NetBird switch profile request, reason: %v", err)
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	command := fmt.Sprintf(`netbird profile select %s && netbird up`, request.Profile)

	username, err := runtime.GetLoggedInUser()
	if err != nil || username == "" {
		out, err := exec.CommandContext(ctx, "bash", "-c", command).CombinedOutput()
		if err != nil {
			log.Printf("[ERROR]: could not switch NetBird profile, reason: %s", string(out))
			return nil, err
		}
	} else {
		args := []string{"-c", command}
		out, err := runtime.RunAsUserWithOutputAndTimeout(username, "bash", args, true, 30*time.Second)
		if err != nil {
			log.Printf("[ERROR]: could not switch NetBird profile, reason: %s", string(out))
			return nil, err
		}
	}

	return report.RetrieveNetbirdInfo()
}

func RefreshInfo(data []byte) (*nats.Netbird, error) {
	return report.RetrieveNetbirdInfo()
}

func getNetbirdBin() string {
	netbirdBin := "/usr/bin/netbird"
	out, err := exec.Command("which", "netbird").Output()
	if err == nil {
		netbirdBin = strings.TrimSpace(string(out))
	}

	return netbirdBin
}
