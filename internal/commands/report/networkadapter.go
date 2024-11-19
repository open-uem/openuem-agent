package report

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/doncicuto/openuem_nats"
)

type networkAdapterInfo struct {
	Index               uint32
	MACAddress          string
	Name                string
	NetConnectionStatus uint16
	Speed               uint64
}

type networkAdapterConfiguration struct {
	DefaultIPGateway     []string
	DHCPEnabled          bool
	DNSDomain            string
	DHCPLeaseExpires     time.Time
	DHCPLeaseObtained    time.Time
	DNSServerSearchOrder []string
	IPAddress            []string
	IPSubnet             []string
}

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

func (r *Report) logNetworkAdapters() {
	fmt.Printf("\n** 📶 Network adapters (Active) *************************************************************************************\n")
	if len(r.NetworkAdapters) > 0 {
		for i, v := range r.NetworkAdapters {
			fmt.Printf("%-40s |  %s \n", "Network Interface", v.Name)
			fmt.Printf("%-40s |  %s \n", "MAC Address", v.MACAddress)
			fmt.Printf("%-40s |  %s \n", "IP Addresses", v.Addresses)
			fmt.Printf("%-40s |  %s \n", "IP Subnet", v.Subnet)
			fmt.Printf("%-40s |  %s \n", "Default gateway", v.DefaultGateway)
			fmt.Printf("%-40s |  %s \n", "DNS Servers", v.DNSServers)
			fmt.Printf("%-40s |  %s \n", "DNS Domain", v.DNSDomain)
			fmt.Printf("%-40s |  %t \n", "DHCP Enabled", v.DHCPEnabled)
			if v.DHCPEnabled {
				fmt.Printf("%-40s |  %s \n", "DHCP Lease Obtained", v.DHCPLeaseObtained)
				fmt.Printf("%-40s |  %s \n", "DHCP Lease Expires", v.DHCPLeaseExpired)
			}
			fmt.Printf("%-40s |  %s \n", "Speed", v.Speed)

			if len(r.NetworkAdapters) > 1 && i+1 != len(r.NetworkAdapters) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No active network adapters found")
	}

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
