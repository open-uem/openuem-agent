package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/doncicuto/openuem-agent/internal/log"
	"github.com/yusufpapurcu/wmi"
)

type NetworkAdapter struct {
	Name              string    `json:"name,omitempty"`
	MACAddress        string    `json:"mac_address,omitempty"`
	Addresses         string    `json:"addresses,omitempty"`
	Subnet            string    `json:"subnet,omitempty"`
	DefaultGateway    string    `json:"default_gateway,omitempty"`
	DNSServers        string    `json:"dns_servers,omitempty"`
	DNSDomain         string    `json:"dns_domain,omitempty"`
	DHCPEnabled       bool      `json:"dhcp_enabled,omitempty"`
	DHCPLeaseObtained time.Time `json:"dhcp_lease_obtained,omitempty"`
	DHCPLeaseExpired  time.Time `json:"dhcp_lease_expired,omitempty"`
	Speed             string    `json:"speed,omitempty"`
}

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

func (a *Agent) getNetworkAdaptersInfo() {
	var err error
	a.Edges.NetworkAdapters, err = getNetworkAdaptersFromWMI()
	if err != nil {
		log.Logger.Printf("[ERROR]: could not get network adapters information from WMI Win32_NetworkAdapter: %v", err)
	} else {
		log.Logger.Printf("[INFO]: network adapters information has been retrieved from WMI Win32_NetworkAdapter")
	}
}

func (a *Agent) logNetworkAdapters() {
	fmt.Printf("\n** 📶 Network adapters (Active) *************************************************************************************\n")
	if len(a.Edges.NetworkAdapters) > 0 {
		for i, v := range a.Edges.NetworkAdapters {
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

			if len(a.Edges.NetworkAdapters) > 1 && i+1 != len(a.Edges.NetworkAdapters) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No active network adapters found")
	}

}

func getNetworkAdaptersFromWMI() ([]NetworkAdapter, error) {
	// Get active network adapters info
	// Ref: https://devblogs.microsoft.com/scripting/using-powershell-to-find-connected-network-adapters/
	// Ref: https://stackoverflow.com/questions/7822708/netaddresses-always-null-in-win32-networkadapter-query
	var networkInfoDst []networkAdapterInfo

	myNetworkAdapters := []NetworkAdapter{}

	namespace := `root\cimv2`
	qNetwork := "SELECT Index, MACAddress, Name, NetConnectionStatus, Speed FROM Win32_NetworkAdapter"
	err := wmi.QueryNamespace(qNetwork, &networkInfoDst, namespace)
	if err != nil {
		return nil, err
	}
	for _, v := range networkInfoDst {
		myNetworkAdapter := NetworkAdapter{}

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

			namespace = `root\cimv2`
			qNetwork := fmt.Sprintf("SELECT DefaultIPGateway, DHCPEnabled, DHCPLeaseExpires, DHCPLeaseObtained, DNSDomain, DNSServerSearchOrder, IPAddress, IPSubnet FROM Win32_NetworkAdapterConfiguration WHERE Index = %d", v.Index)
			err = wmi.QueryNamespace(qNetwork, &networkAdapterDst, namespace)
			if err != nil {
				return nil, err
			}

			if len(networkAdapterDst) != 1 {
				return nil, fmt.Errorf("got wrong network adapter configuration result set")
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

			myNetworkAdapters = append(myNetworkAdapters, myNetworkAdapter)
		}
	}

	return myNetworkAdapters, nil
}
