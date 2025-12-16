//go:build windows

package report

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func RetrieveNetbirdInfo() (*nats.Netbird, error) {
	data := nats.Netbird{}

	netbirdBin := "C:\\Program Files\\NetBird\\netbird.exe"
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
		out, err = exec.Command(netbirdBin, "service", "status").CombinedOutput()
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
			out, err = exec.Command(netbirdBin, "status", "--json").CombinedOutput()
			if err != nil {
				log.Printf("[ERROR]: could not execute NetBird service status, reason: %s", string(out))
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
		args := []string{"profile", "list"}
		out, err = runtime.RunAsUserWithOutput(netbirdBin, args)
		if err != nil {
			log.Printf("[ERROR]: could not get NetBird profiles, reason: %s", string(out))
			return nil, err
		}

		profiles := []string{}
		for i, p := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if i == 0 {
				continue
			}
			items := strings.Split(p, " ")
			if len(items) > 1 {
				profiles = append(profiles, items[1])
			}
		}
		data.Profiles = profiles
	}

	return &data, nil
}
