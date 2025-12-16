//go:build darwin

package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func RetrieveNetbirdInfo() (*nats.Netbird, error) {
	data := nats.Netbird{}

	netbirdBin := "/usr/local/bin/netbird"
	s := NetBirdOverview{}

	_, err := os.Stat(netbirdBin)
	if err == nil {
		data.Installed = true

		// Get NetBird version
		out, err := exec.Command(netbirdBin, "version").CombinedOutput()
		if err != nil {
			log.Printf("[ERROR]: could not get NetBird version, reason: %v", string(out))
			return nil, err
		}
		data.Version = strings.TrimSpace(string(out))

		// Check NetBird service version
		data.ServiceStatus = "netbird.service_not_installed"
		command := `export LC_ALL=C && /usr/local/bin/netbird service status`
		out, err = exec.Command("bash", "-c", command).CombinedOutput()
		if err != nil {
			log.Printf("[ERROR]: could not check NetBird service status, reason: %s", string(out))
			return nil, err
		}

		status := strings.ToLower(string(out))
		serviceRunning := strings.Contains(status, "running")
		if serviceRunning {
			data.ServiceStatus = "netbird.service_running"
		}
		if strings.Contains(status, "stopped") {
			data.ServiceStatus = "netbird.service_stopped"
		}

		if serviceRunning {
			// Check NetBird status
			command := fmt.Sprintf("%s status --json", netbirdBin)

			username, err := runtime.GetLoggedInUser()
			if err != nil {
				return nil, errors.New("could not check if a user is logged in")
			}

			if username == "" {
				username = "root"
			}
			args := []string{"-c", command}
			out, err := runtime.RunAsUserWithOutputAndTimeout(username, "bash", args, true, 60*time.Second)
			if err != nil {
				log.Printf("[ERROR]: could not switch NetBird profile, reason: %s", string(out))
				return nil, err
			}

			if err := json.Unmarshal(out, &s); err == nil {
				data.IP = s.IP
				data.Profile = s.ProfileName
				data.ManagementConnected = s.ManagementState.Connected
				data.ManagementURL = s.ManagementState.URL
				data.SignalConnected = s.SignalState.Connected
				data.SignalURL = s.SignalState.URL
				data.PeersConnected = s.Peers.Connected
				data.PeersTotal = s.Peers.Total
				data.SSHEnabled = s.SSHServerState.Enabled

				if len(s.NSServerGroups) > 0 {
					dnsServers := []string{}
					for _, nsg := range s.NSServerGroups {
						dnsServers = append(dnsServers, nsg.Servers...)
					}
					data.DNSServers = dnsServers
				}
			}
		}

		// Check applied profiles
		username, err := runtime.GetLoggedInUser()
		if err != nil || username == "" {
			command = `export LC_ALL=C && /usr/local/bin/netbird profile list | awk 'NR>1 {print $2}'`
			out, err = exec.Command("bash", "-c", command).CombinedOutput()
			if err != nil {
				log.Printf("[ERROR]: could not get NetBird profiles, reason: %s", string(out))
				return nil, err
			}
			data.Profiles = strings.Split(strings.TrimSpace(string(out)), "\n")
		} else {
			args := []string{"-c", `export LC_ALL=C && /usr/local/bin/netbird profile list | awk 'NR>1 {print $2}'`}
			out, err := runtime.RunAsUserWithOutput(username, "bash", args, true)
			if err != nil {
				log.Printf("[ERROR]: could not get NetBird profiles, reason: %s", string(out))
				return nil, err
			}
			data.Profiles = strings.Split(strings.TrimSpace(string(out)), "\n")
		}
	}

	return &data, nil
}
