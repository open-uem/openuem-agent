//go:build windows

package report

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	openuem_nats "github.com/open-uem/nats"
)

type networkAdapterInfo struct {
	Index               uint32
	InterfaceIndex      uint32
	MACAddress          string
	Name                string
	NetConnectionStatus uint16
	Speed               uint64
	PNPDeviceID         string
}

type defaultRouteInfo struct {
	InterfaceIndex uint32
	Metric1        int32
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

func (r *Report) getNetworkAdaptersFromWMI() error {
	// Get active network adapters info
	// Ref: https://devblogs.microsoft.com/scripting/using-powershell-to-find-connected-network-adapters/
	// Ref: https://stackoverflow.com/questions/7822708/netaddresses-always-null-in-win32-networkadapter-query
	var networkInfoDst []networkAdapterInfo

	namespace := `root\cimv2`
	qNetwork := "SELECT Index, InterfaceIndex, MACAddress, Name, NetConnectionStatus, Speed, PNPDeviceID FROM Win32_NetworkAdapter"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qNetwork, &networkInfoDst, namespace)
	if err != nil {
		return err
	}
	connectedAdapters := []struct {
		index   uint32
		adapter openuem_nats.NetworkAdapter
	}{}
	for _, v := range networkInfoDst {
		myNetworkAdapter := openuem_nats.NetworkAdapter{}

		if v.NetConnectionStatus == 2 {
			interfaceIndex := v.InterfaceIndex
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
			// Virtual adapters (VPN tunnels, hypervisors, etc.) are created under
			// the device tree ROOT or SWD branches instead of a hardware bus.
			myNetworkAdapter.Virtual = strings.HasPrefix(v.PNPDeviceID, `ROOT\`) || strings.HasPrefix(v.PNPDeviceID, `SWD\`)

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

			connectedAdapters = append(connectedAdapters, struct {
				index   uint32
				adapter openuem_nats.NetworkAdapter
			}{index: interfaceIndex, adapter: myNetworkAdapter})
		}
	}

	// Order adapters so the interface owning the best default route (lowest
	// metric) comes first. The report uses the first connected adapter that has
	// a gateway, and WMI order is arbitrary — virtual adapters (VPNs, tunnels)
	// used to win over the physical NIC that actually serves the default route.
	metricByInterface := map[uint32]int32{}
	var defaultRoutesDst []defaultRouteInfo
	qRoutes := "SELECT InterfaceIndex, Metric1 FROM Win32_IP4RouteTable WHERE Destination = '0.0.0.0' AND Mask = '0.0.0.0'"
	if err := WMIQueryWithContext(ctx, qRoutes, &defaultRoutesDst, namespace); err == nil {
		for _, route := range defaultRoutesDst {
			if metric, ok := metricByInterface[route.InterfaceIndex]; !ok || route.Metric1 < metric {
				metricByInterface[route.InterfaceIndex] = route.Metric1
			}
		}
	}

	sort.SliceStable(connectedAdapters, func(i, j int) bool {
		metricI, okI := metricByInterface[connectedAdapters[i].index]
		metricJ, okJ := metricByInterface[connectedAdapters[j].index]
		if okI != okJ {
			return okI
		}
		if okI && okJ {
			return metricI < metricJ
		}
		return false
	})

	for _, ca := range connectedAdapters {
		r.NetworkAdapters = append(r.NetworkAdapters, ca.adapter)
	}

	return nil
}
