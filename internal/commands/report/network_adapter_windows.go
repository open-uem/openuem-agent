//go:build windows

package report

import (
	"context"
	"fmt"
	"log"
	"strings"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getNetworkAdaptersInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: network adapters info has been requested")
	}

	err := r.getNetworkAdaptersFromWMI()
	if err != nil {
		log.Printf("[ERROR]: could not get network adapters information from WMI Win32_NetworkAdapter: %v", err)
		return err
	} else {
		log.Printf("[INFO]: network adapters information has been retrieved from WMI Win32_NetworkAdapter")
	}
	return nil
}

func (r *Report) getNetworkAdaptersFromWMI() error {
	// Get active network adapters info
	// Ref: https://devblogs.microsoft.com/scripting/using-powershell-to-find-connected-network-adapters/
	// Ref: https://stackoverflow.com/questions/7822708/netaddresses-always-null-in-win32-networkadapter-query
	var networkInfoDst []networkAdapterInfo

	namespace := `root\cimv2`
	qNetwork := "SELECT Index, MACAddress, Name, NetConnectionStatus, Speed FROM Win32_NetworkAdapter"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qNetwork, &networkInfoDst, namespace)
	if err != nil {
		return err
	}
	for _, v := range networkInfoDst {
		myNetworkAdapter := openuem_nats.NetworkAdapter{}

		if v.NetConnectionStatus == 2 {
			var networkAdapterDst []networkAdapterConfiguration

			speed := v.Speed / 1_000_000
			speedInUnits := "Mbps"
			isGbps := v.Speed/1000_000_000 > 0
			if isGbps {
				speedInUnits = "Gbps"
				speed = speed / 1000
			}
			myNetworkAdapter.Speed = fmt.Sprintf("%d %s", speed, speedInUnits)
			myNetworkAdapter.Name = v.Name
			myNetworkAdapter.MACAddress = v.MACAddress

			// This query would not be acceptable in general as it could lead to sql injection, but we're using a where condition using a
			// index value retrieved by WMI it's not user generated input
			namespace = `root\cimv2`
			qNetwork := fmt.Sprintf("SELECT DefaultIPGateway, DHCPEnabled, DHCPLeaseExpires, DHCPLeaseObtained, DNSDomain, DNSServerSearchOrder, IPAddress, IPSubnet FROM Win32_NetworkAdapterConfiguration WHERE Index = %d", v.Index)

			err = WMIQueryWithContext(ctx, qNetwork, &networkAdapterDst, namespace)
			if err != nil {
				return err
			}

			if len(networkAdapterDst) != 1 {
				return fmt.Errorf("got wrong network adapter configuration result set")
			}
			v := &networkAdapterDst[0]

			if len(v.IPAddress) > 0 {
				myNetworkAdapter.Addresses = v.IPAddress[0]
			}

			if len(v.IPSubnet) > 0 {
				myNetworkAdapter.Subnet = v.IPSubnet[0]
			}

			myNetworkAdapter.DefaultGateway = strings.Join(v.DefaultIPGateway, ", ")
			myNetworkAdapter.DNSServers = strings.Join(v.DNSServerSearchOrder, ", ")
			myNetworkAdapter.DNSDomain = v.DNSDomain
			myNetworkAdapter.DHCPEnabled = v.DHCPEnabled
			if v.DHCPEnabled {
				myNetworkAdapter.DHCPLeaseObtained = v.DHCPLeaseObtained.Local()
				myNetworkAdapter.DHCPLeaseExpired = v.DHCPLeaseExpires.Local()
			}

			r.NetworkAdapters = append(r.NetworkAdapters, myNetworkAdapter)
		}
	}

	return nil
}
