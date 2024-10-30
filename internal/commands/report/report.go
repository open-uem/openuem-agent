package report

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/doncicuto/openuem_nats"
	"github.com/scjalliance/comshim"
	"golang.org/x/sys/windows"
)

type Report struct {
	openuem_nats.AgentReport
}

func RunReport(agentId string) *Report {
	var wg sync.WaitGroup
	var err error
	// Prepare COM
	comshim.Add(1)
	defer comshim.Done()

	report := Report{}
	report.AgentID = agentId
	report.OS = "windows"
	report.Version = "0.1.0"
	report.ExecutionTime = time.Now()

	report.Hostname, err = windows.ComputerName()
	if err != nil {
		log.Printf("[ERROR]: could not get computer name: %v", err)
		report.Hostname = "UNKNOWN"
	}

	// These operations will be run using goroutines
	wg.Add(1)
	go func() {
		defer wg.Done()
		report.getComputerInfo()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		report.getOperatingSystemInfo()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		report.getOSInfo()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getMonitorsInfo(); err != nil {
			// Retry
			report.getMonitorsInfo()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		report.getLogicalDisksInfo()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		report.getPrintersInfo()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		report.getSharesInfo()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		report.getAntivirusInfo()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		report.getSystemUpdateInfo()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		report.getNetworkAdaptersInfo()
		// Get network adapter with default gateway and set its ip address and MAC as the report IP/MAC address
		for _, n := range report.NetworkAdapters {
			if n.DefaultGateway != "" {
				report.IP = n.Addresses
				report.MACAddress = n.MACAddress
				break
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		report.getApplicationsInfo()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		report.getVNCInfo()
	}()

	wg.Wait()

	return &report
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
