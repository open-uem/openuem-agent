package report

import (
	"fmt"
	"time"
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
