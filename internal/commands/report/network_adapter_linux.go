//go:build linux

package report

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os/exec"
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

		// Ignore interfaces that are not attached to a NIC
		// if i.Name == "lo" || state != 1 || strings.HasPrefix(i.Name, "veth") {
		// 	continue
		// }

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
			if err != nil {
				log.Printf("[ERROR]: could not get default gateway, %v\n", err)
			}
		}

		r.NetworkAdapters = append(r.NetworkAdapters, myNetworkAdapter)
	}

	// for _, v := range networkInfoDst {
	// 	myNetworkAdapter := openuem_nats.NetworkAdapter{}

	// 	if v.NetConnectionStatus == 2 {
	// 		var networkAdapterDst []networkAdapterConfiguration

	// 		speed := v.Speed / 1_000_000
	// 		speedInUnits := "Mbps"
	// 		isGbps := v.Speed/1000_000_000 > 0
	// 		if isGbps {
	// 			speedInUnits = "Gbps"
	// 			speed = speed / 1000
	// 		}
	// 		myNetworkAdapter.Speed = fmt.Sprintf("%d %s", speed, speedInUnits)
	// 		myNetworkAdapter.Name = v.Name
	// 		myNetworkAdapter.MACAddress = v.MACAddress

	// 		// This query would not be acceptable in general as it could lead to sql injection, but we're using a where condition using a
	// 		// index value retrieved by WMI it's not user generated input
	// 		namespace = `root\cimv2`
	// 		qNetwork := fmt.Sprintf("SELECT DefaultIPGateway, DHCPEnabled, DHCPLeaseExpires, DHCPLeaseObtained, DNSDomain, DNSServerSearchOrder, IPAddress, IPSubnet FROM Win32_NetworkAdapterConfiguration WHERE Index = %d", v.Index)

	// 		err = WMIQueryWithContext(ctx, qNetwork, &networkAdapterDst, namespace)
	// 		if err != nil {
	// 			return err
	// 		}

	// 		if len(networkAdapterDst) != 1 {
	// 			return fmt.Errorf("got wrong network adapter configuration result set")
	// 		}
	// 		v := &networkAdapterDst[0]

	// 		if len(v.IPAddress) > 0 {
	// 			myNetworkAdapter.Addresses = v.IPAddress[0]
	// 		}

	// 		if len(v.IPSubnet) > 0 {
	// 			myNetworkAdapter.Subnet = v.IPSubnet[0]
	// 		}

	// 		myNetworkAdapter.DefaultGateway = strings.Join(v.DefaultIPGateway, ", ")
	// 		myNetworkAdapter.DNSServers = strings.Join(v.DNSServerSearchOrder, ", ")
	// 		myNetworkAdapter.DNSDomain = v.DNSDomain
	// 		myNetworkAdapter.DHCPEnabled = v.DHCPEnabled
	// 		if v.DHCPEnabled {
	// 			myNetworkAdapter.DHCPLeaseObtained = v.DHCPLeaseObtained.Local()
	// 			myNetworkAdapter.DHCPLeaseExpired = v.DHCPLeaseExpires.Local()
	// 		}

	// 		r.NetworkAdapters = append(r.NetworkAdapters, myNetworkAdapter)
	// 	}
	// }

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
