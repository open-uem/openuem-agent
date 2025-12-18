//go:build linux

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

	"github.com/open-uem/nats"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/report"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func Install() (*openuem_nats.Netbird, error) {
	var err error

	command := "curl -fsSL https://pkgs.netbird.io/install.sh | sh"
	c1 := exec.Command("bash", "-c", command)

	if hasGraphicalDesktop() {
		c1.Env = os.Environ()
		c1.Env = append(c1.Env, "XDG_CURRENT_DESKTOP=OpenUEM")
	}

	out, err := c1.CombinedOutput()
	if err != nil {
		log.Printf("[ERROR]: could not install the NetBird client, reason: %s", string(out))
		return nil, errors.New(string(out))
	}

	return report.RetrieveNetbirdInfo()
}

func Uninstall() error {
	var err error

	command := "curl -fsSL https://downloads.openuem.eu/netbird/netbird_uninstall.sh | sh"
	c1 := exec.Command("bash", "-c", command)
	desktop, err := runtime.GetUserEnv("XDG_CURRENT_DESKTOP")
	if err == nil {
		c1.Env = os.Environ()
		c1.Env = append(c1.Env, fmt.Sprintf("XDG_CURRENT_DESKTOP=%s", desktop))
	}

	out, err := c1.CombinedOutput()
	if err != nil {
		log.Printf("[ERROR]: could not install the NetBird client, reason: %s", string(out))
		return errors.New(string(out))
	}

	return nil
}

func SwitchProfile(request openuem_nats.NetbirdSettings) (*nats.Netbird, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	command := fmt.Sprintf(`netbird profile select %s --management-url %s && netbird up --management-url %s`, request.Profile, request.ManagementURL, request.ManagementURL)

	username, err := runtime.GetLoggedInUser()
	if err != nil || username == "" {
		out, err := exec.CommandContext(ctx, "bash", "-c", command).CombinedOutput()
		if err != nil {
			log.Printf("[ERROR]: could not switch NetBird profile, reason: %s", string(out))
			return nil, err
		}
	} else {
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
	request := openuem_nats.NetbirdSettings{}
	if err := json.Unmarshal(data, &request); err != nil {
		log.Printf("[ERROR]: could not unmarshal the NetBird register request, reason: %v", err)
		return nil, err
	}

	bin := getNetbirdBin()

	// First, we must set the connection down
	if err := exec.Command(bin, "down", "--management-url", request.ManagementURL).Run(); err != nil {
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

func NetbirdUp(data []byte) (*nats.Netbird, error) {
	request := openuem_nats.NetbirdSettings{}
	if err := json.Unmarshal(data, &request); err != nil {
		log.Printf("[ERROR]: could not unmarshal the NetBird request, reason: %v", err)
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	command := fmt.Sprintf(`netbird up --management-url %s`, request.ManagementURL)

	username, err := runtime.GetLoggedInUser()
	if err != nil || username == "" {
		out, err := exec.CommandContext(ctx, "bash", "-c", command).CombinedOutput()
		if err != nil {
			log.Printf("[ERROR]: could not execute netbird up, reason: %s", string(out))
			return nil, err
		}
	} else {
		args := []string{"-c", command}
		out, err := runtime.RunAsUserWithOutputAndTimeout(username, "bash", args, true, 60*time.Second)
		if err != nil {
			log.Printf("[ERROR]: could not execute netbird up, reason: %s", string(out))
			return nil, err
		}
	}

	return report.RetrieveNetbirdInfo()
}

func NetbirdDown(data []byte) (*nats.Netbird, error) {
	request := openuem_nats.NetbirdSettings{}
	if err := json.Unmarshal(data, &request); err != nil {
		log.Printf("[ERROR]: could not unmarshal the NetBird request, reason: %v", err)
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	command := fmt.Sprintf(`netbird down --management-url %s`, request.ManagementURL)

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
	netbirdBin := "/usr/bin/netbird"
	out, err := exec.Command("which", "netbird").Output()
	if err == nil {
		netbirdBin = strings.TrimSpace(string(out))
	}

	return netbirdBin
}

func hasGraphicalDesktop() bool {
	hasXOrg := false
	hasXWayland := false

	if err := exec.Command("bash", "-c", "type Xorg").Run(); err == nil {
		hasXOrg = true
	}

	if err := exec.Command("bash", "-c", "type Xwayland").Run(); err == nil {
		hasXWayland = true
	}

	return hasXOrg || hasXWayland
}
