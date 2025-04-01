//go:build linux

package report

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	openuem_nats "github.com/open-uem/nats"
	"github.com/safchain/ethtool"
	"github.com/zcalusic/sysinfo"
)

func (r *Report) getNetworkAdaptersInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: network adapters info has been requested")
	}

	err := r.getNetworkAdaptersFromLinux()
	if err != nil {
		log.Printf("[ERROR]: could not get network adapters information from ethtool: %v", err)
		return err
	} else {
		log.Printf("[INFO]: network adapters information has been retrieved from ethtool")
	}
	return nil
}

func (r *Report) getNetworkAdaptersFromLinux() error {
	var si sysinfo.SysInfo

	detectedNICs := []string{}

	si.GetSysInfo()
	for _, n := range si.Network {
		detectedNICs = append(detectedNICs, n.Name)
	}

	ethHandle, err := ethtool.NewEthtool()
	if err != nil {
		log.Printf("[ERROR]: could not initialize ethtool, %v\n", err)
		return err
	}
	defer ethHandle.Close()

	ifaces, err := net.Interfaces()
	if err != nil {
		log.Printf("[ERROR]: could not get linux interfaces, %v\n", err)
		return err
	}
	for _, i := range ifaces {
		myNetworkAdapter := openuem_nats.NetworkAdapter{}
		myNetworkAdapter.Name = i.Name

		state, err := ethHandle.LinkState(i.Name)
		if err != nil {
			log.Printf("[ERROR]: could not get interface link state, %v\n", err)
			return err
		}

		if !slices.Contains(detectedNICs, i.Name) || state != 1 {
			continue
		}

		myNetworkAdapter.MACAddress = i.HardwareAddr.String()
		ethSettings, err := ethtool.CmdGetMapped(i.Name)
		if err != nil {
			log.Printf("[ERROR]: could not get ip addressed assigned to interface, %v\n", err)
		} else {
			speedInBps := ethSettings["Speed"]
			speedInUnits := "Mbps"
			isGbps := speedInBps/1000_000_000 > 0
			if isGbps {
				speedInUnits = "Gbps"
				speedInBps = speedInBps / 1000
			}
			myNetworkAdapter.Speed = fmt.Sprintf("%d %s", speedInBps, speedInUnits)

			iface, err := net.InterfaceByName(i.Name)
			if err != nil {
				log.Printf("[ERROR]: could not get ip addressed assigned to interface, %v\n", err)
				continue
			}

			addresses, err := iface.Addrs()
			if err != nil {
				log.Printf("[ERROR]: could not get ip addressed assigned to interface, %v\n", err)
				continue
			}

			strAddresses := []string{}
			subnets := []string{}
			for _, a := range addresses {
				ipv4Addr := a.(*net.IPNet).IP.To4()
				if ipv4Addr != nil {
					strAddresses = append(strAddresses, ipv4Addr.String())
					subnetMask := a.(*net.IPNet).Mask
					subnets = append(subnets, fmt.Sprintf("%d.%d.%d.%d", subnetMask[0], subnetMask[1], subnetMask[2], subnetMask[3]))
				}
			}

			myNetworkAdapter.Addresses = strings.Join(strAddresses, ",")
			myNetworkAdapter.Subnet = strings.Join(subnets, ",")
			myNetworkAdapter.DefaultGateway, err = getDefaultGateway()
			myNetworkAdapter.DNSServers = getDNSservers()
			myNetworkAdapter.DNSDomain = getDNSDomain()

			if len(strAddresses) > 0 {
				myNetworkAdapter.DHCPEnabled = isDHCPEnabled(strAddresses[0])
			}

			if err != nil {
				log.Printf("[ERROR]: could not get default gateway, %v\n", err)
			}
		}

		r.NetworkAdapters = append(r.NetworkAdapters, myNetworkAdapter)
	}

	return nil
}

// Reference: https://github.com/net-byte/go-gateway/blob/main/gateway_linux.go
// License: https://github.com/net-byte/go-gateway/blob/main/LICENSE
func getDefaultGateway() (string, error) {
	cmd := "route -n | grep 'UG[ \t]' | awk 'NR==1{print $2}'"
	out, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		return "", err
	}

	commandOutput := strings.TrimSpace(string(out))
	ipv4 := net.ParseIP(commandOutput)
	if ipv4 == nil {
		return "", errors.New("could not parse route command response")
	}
	return commandOutput, nil
}

func getDNSservers() string {
	out, err := exec.Command("resolvectl", "status").Output()
	if err == nil {
		reg := regexp.MustCompile(`DNS Servers: (.*)`)
		matches := reg.FindAllStringSubmatch(string(out), -1)
		for _, v := range matches {
			return v[1]
		}
	} else {
		log.Println("[ERROR]: resolvectl status failed or not found")
	}

	file, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		log.Println("[ERROR]: could not read /etc/resolv.conf")
		return ""
	}

	dnsServers := []string{}
	reg := regexp.MustCompile(`nameserver \s*(.*?)\s`)
	matches := reg.FindAllStringSubmatch(string(file), -1)
	for _, v := range matches {
		dnsServers = append(dnsServers, v[1])
	}
	return strings.Join(dnsServers, ",")
}

func isDHCPEnabled(ip string) bool {
	command := fmt.Sprintf("ip -o address | grep %s | grep dynamic | wc -l", ip)
	out, err := exec.Command("bash", "-c", command).Output()
	if err != nil {
		log.Println("[ERROR]: could not check if IP address has been set via DHCP")
		return false
	}

	return strings.TrimSpace(string(out)) == "1"
}

func getDNSDomain() string {
	out, err := exec.Command("hostname", "-d").Output()
	if err != nil {
		log.Println("[ERROR]: could not get the domain")
		return ""
	}

	return strings.TrimSpace(string(out))
}
