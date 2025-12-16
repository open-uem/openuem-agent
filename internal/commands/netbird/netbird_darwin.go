//go:build darwin

package netbird

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/report"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func Install() (*openuem_nats.Netbird, error) {
	var err error

	command := "curl -fsSL https://pkgs.netbird.io/install.sh | sh"
	c1 := exec.Command("bash", "-c", command)

	out, err := c1.CombinedOutput()
	if err != nil {
		log.Printf("[ERROR]: could not install the NetBird client, reason: %s", string(out))
		return nil, errors.New(string(out))
	}

	// it seems that the netbird install doesn't create the /var/lib/netbird/default.json
	// in certain circumstances
	if _, err := os.Stat("/var/lib/netbird/default.json"); err != nil {
		if err := os.MkdirAll("/var/lib/netbird", 0750); err != nil {
			return nil, err
		}

		f, err := os.Create("/var/lib/netbird/default.json")
		if err != nil {
			return nil, err
		}
		defer func() { f.Close() }()

		data, err := json.Marshal(struct{}{})
		if err != nil {
			return nil, err
		}

		if _, err := f.Write(data); err != nil {
			return nil, err
		}

	}

	return report.RetrieveNetbirdInfo()
}

func Uninstall() error {
	var err error

	command := "curl -fsSL https://downloads.openuem.eu/netbird/netbird_uninstall.sh | sh"
	c1 := exec.Command("bash", "-c", command)

	out, err := c1.CombinedOutput()
	if err != nil {
		log.Printf("[ERROR]: could not install the NetBird client, reason: %s", string(out))
		return errors.New(string(out))
	}

	return nil
}

func SwitchProfile(request openuem_nats.NetbirdSwitchProfile) (*openuem_nats.Netbird, error) {
	command := fmt.Sprintf(`/usr/local/bin/netbird profile select %s && /usr/local/bin/netbird up`, request.Profile)

	username, err := runtime.GetLoggedInUser()
	if err == nil {
		if username == "" {
			username = "root"
		}
		args := []string{"-c", command}
		out, err := runtime.RunAsUserWithOutputAndTimeout(username, "bash", args, true, 2*time.Minute)
		if err != nil {
			log.Printf("[ERROR]: could not switch NetBird profile, reason: %s", string(out))
			return nil, err
		}
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

	command := fmt.Sprintf("%s up --setup-key %s --management-url %s", bin, request.OneOffKey, request.ManagementURL)

	username, err := runtime.GetLoggedInUser()
	if err != nil || username == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		out, err := exec.CommandContext(ctx, "bash", "-c", command).CombinedOutput()
		if err != nil {
			log.Printf("[ERROR]: could not execute netbird up, reason: %s", string(out))
			return nil, err
		}
	} else {
		args := []string{"-c", command}
		out, err := runtime.RunAsUserWithOutputAndTimeout(username, "bash", args, true, 30*time.Second)
		if err != nil {
			log.Printf("[ERROR]: could not execute netbird up to register the client, reason: %s", string(out))
			return nil, err
		}
	}

	return report.RetrieveNetbirdInfo()
}

func NetbirdUp(data []byte) (*openuem_nats.Netbird, error) {
	command := `/usr/local/bin/netbird up`

	username, err := runtime.GetLoggedInUser()
	if err != nil || username == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		out, err := exec.CommandContext(ctx, "bash", "-c", command).CombinedOutput()
		if err != nil {
			log.Printf("[ERROR]: could not execute netbird up, reason: %s", string(out))
			return nil, err
		}
	} else {
		args := []string{"-c", command}
		out, err := runtime.RunAsUserWithOutputAndTimeout(username, "bash", args, true, 30*time.Second)
		if err != nil {
			log.Printf("[ERROR]: could not execute netbird up, reason: %s", string(out))
			return nil, err
		}
	}

	return report.RetrieveNetbirdInfo()
}

func NetbirdDown(data []byte) (*openuem_nats.Netbird, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	command := `/usr/local/bin/netbird down`

	username, err := runtime.GetLoggedInUser()
	if err != nil || username == "" {
		out, err := exec.CommandContext(ctx, "bash", "-c", command).CombinedOutput()
		if err != nil {
			log.Printf("[ERROR]: could not execute netbird down, reason: %s", string(out))
			return nil, err
		}
	} else {
		args := []string{"-c", command}
		out, err := runtime.RunAsUserWithOutputAndTimeout(username, "bash", args, true, 60*time.Second)
		if err != nil {
			log.Printf("[ERROR]: could not execute netbird down, reason: %s", string(out))
			return nil, err
		}
	}

	return report.RetrieveNetbirdInfo()
}

func getNetbirdBin() string {
	netbirdBin := "/usr/local/bin/netbird"
	out, err := exec.Command("which", "netbird").Output()
	if err == nil {
		netbirdBin = strings.TrimSpace(string(out))
	}

	return netbirdBin
}
