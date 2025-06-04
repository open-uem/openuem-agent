//go:build darwin

package report

import (
	"encoding/json"
	"log"
	"os/exec"
	"strings"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getNetworkAdaptersInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: network adapters info has been requested")
	}

	err := r.getNetworkAdaptersFromMac()
	if err != nil {
		log.Printf("[ERROR]: could not get network adapters information: %v", err)
		return err
	} else {
		log.Printf("[INFO]: network adapters information has been retrieved")
	}
	return nil
}

func (r *Report) getNetworkAdaptersFromMac() error {
	var networkData SPNetworkDataType
	r.NetworkAdapters = []openuem_nats.NetworkAdapter{}

	out, err := exec.Command("system_profiler", "-json", "SPNetworkDataType").Output()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(out, &networkData); err != nil {
		return err
	}

	for _, i := range networkData.SPNetworkDataType {
		if len(i.IPAddress) == 0 {
			continue
		}

		myNetworkAdapter := openuem_nats.NetworkAdapter{}
		myNetworkAdapter.Name = i.Interface
		myNetworkAdapter.MACAddress = i.Ethernet.MACAddress
		myNetworkAdapter.Speed = " - "
		myNetworkAdapter.Addresses = strings.Join(i.IPV4.Addresses, ",")
		myNetworkAdapter.Subnet = strings.Join(i.IPV4.SubnetMasks, ",")
		myNetworkAdapter.DefaultGateway = i.IPV4.Router
		myNetworkAdapter.DNSServers = strings.Join(i.DNS.ServerAddresses, ",")
		myNetworkAdapter.DNSDomain = getDNSDomain()
		myNetworkAdapter.DHCPEnabled = i.IPV4.ConfigMethod == "DHCP"
		r.NetworkAdapters = append(r.NetworkAdapters, myNetworkAdapter)
	}

	return nil
}

func getDNSDomain() string {
	out, err := exec.Command("hostname", "-d").Output()
	if err != nil {
		log.Println("[ERROR]: could not get the domain")
		return ""
	}

	return strings.TrimSpace(string(out))
}

type SPNetworkDataType struct {
	SPNetworkDataType []NetworkDataType `json:"SPNetworkDataType"`
}

type NetworkDataType struct {
	Name                  string   `json:"_name"`
	Hardware              string   `json:"hardware"`
	Interface             string   `json:"interface"`
	SPNetworkServiceOrder int      `json:"spnetwork_service_order"`
	NetworkType           string   `json:"type"`
	DNS                   DNS      `json:"DNS"`
	IPAddress             []string `json:"ip_address"`
	Ethernet              Ethernet `json:"Ethernet"`
	IPV4                  IPV4     `json:"IPV4"`
	IPV6                  IPV6     `json:"IPV6"`
}

type IPV4 struct {
	ConfigMethod               string            `json:"ConfigMethod"`
	AdditionalRoutes           []AdditionalRoute `json:"AdditionalRoutes"`
	Addresses                  []string          `json:"Addresses"`
	ARPResolvedHardwareAddress string            `json:"ARPResolvedHardwareAddress"`
	ARPResolvedIPAddress       string            `json:"ARPResolvedIPAddress"`
	ConfirmedInterfaceName     string            `json:"ConfirmedInterfaceName"`
	InterfaceName              string            `json:"InterfaceName"`
	NetworkSignature           string            `json:"NetworkSignature"`
	Router                     string            `json:"Router"`
	SubnetMasks                []string          `json:"SubnetMasks"`
}

type IPV6 struct {
	ConfigMethod string `json:"ConfigMethod"`
}

type Proxies struct {
	FTPPassive     string   `json:"FTPPassive"`
	ExceptionsList []string `json:"ExceptionsList"`
}

type DNS struct {
	ServerAddresses []string `json:"ServerAddresses"`
}

type Ethernet struct {
	MACAddress   string   `json:"MAC Address"`
	MediaOptions []string `json:"MediaOptions"`
	MediaSubType string   `json:"MediaSubType"`
}

type AdditionalRoute struct {
	DestinationAddress string `json:"DestinationAddress"`
	SubnetMask         string `json:"SubnetMask"`
}
