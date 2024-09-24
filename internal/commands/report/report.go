package report

import (
	"fmt"
	"log"

	"github.com/doncicuto/openuem_nats"
	"github.com/scjalliance/comshim"
	"golang.org/x/sys/windows"
)

type Report struct {
	openuem_nats.AgentReport
}

func RunReport() {
	var err error
	// Prepare COM
	comshim.Add(1)
	defer comshim.Done()

	report := Report{}
	report.OS = "windows"

	report.Hostname, err = windows.ComputerName()
	if err != nil {
		log.Printf("[ERROR]: could not get computer name: %v", err)
		report.Hostname = "UNKNOWN"
	}

	// These operations don't benefit from goroutines
	report.getComputerInfo()
	report.getOperatingSystemInfo()
	report.getMonitorsInfo()
	report.getLogicalDisksInfo()
	report.getPrintersInfo()
	report.getSharesInfo()
	report.getAntivirusInfo()
	report.getSystemUpdateInfo()
	report.getNetworkAdaptersInfo()

	// Get network adapter with default gateway and set its ip address as the report IP address
	for _, n := range report.NetworkAdapters {
		if n.DefaultGateway != "" {
			report.IP = n.Addresses
			break
		}
	}

	report.getApplicationsInfo()

}

func (r *Report) Print() {
	fmt.Printf("\n** 🕵  Agent *********************************************************************************************************\n")
	fmt.Printf("%-40s |  %s\n", "Computer Name", r.Hostname)
	fmt.Printf("%-40s |  %s\n", "IP address", r.IP)
	fmt.Printf("%-40s |  %s\n", "Operating System", r.OS)

	r.logComputer()
	r.logOS()
	r.logLogicalDisks()
	r.logMonitors()
	r.logPrinters()
	r.logShares()
	r.logAntivirus()
	r.logSystemUpdate()
	r.logNetworkAdapters()
	r.logApplications()
}
