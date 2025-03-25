//go:build linux

package report

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

	openuem_nats "github.com/open-uem/nats"
	"github.com/zcalusic/sysinfo"
)

func RunReport(agentId string, enabled, debug bool, vncProxyPort, sftpPort string) (*Report, error) {
	var si sysinfo.SysInfo
	var wg sync.WaitGroup
	var err error

	if debug {
		log.Println("[DEBUG]: preparing report info")
	}

	// Get system info
	si.GetSysInfo()

	report := Report{}
	report.AgentID = agentId
	report.OS = si.OS.Vendor
	report.SFTPPort = sftpPort
	report.VNCProxyPort = vncProxyPort
	report.CertificateReady = isCertificateReady()
	report.Enabled = enabled

	// Check if a restart is still required
	// Get conf file
	// configFile := openuem_utils.GetAgentConfigFile()

	// Open ini file
	// cfg, err := ini.Load(configFile)
	// if err != nil {
	// 	return nil, err
	// }

	// key, err := cfg.Section("Agent").GetKey("RestartRequired")
	// if err != nil {
	// 	log.Println("[ERROR]: could not read RestartRequired from INI")
	// 	return nil, err
	// }

	// report.RestartRequired, err = key.Bool()
	// if err != nil {
	// 	log.Println("[ERROR]: could not parse RestartRequired")
	// 	return nil, err
	// }

	report.Release = openuem_nats.Release{
		Version: VERSION,
		Arch:    runtime.GOARCH,
		Os:      runtime.GOOS,
		Channel: CHANNEL,
	}
	report.ExecutionTime = time.Now()

	report.Hostname = strings.ToUpper(si.Node.Hostname)
	if report.Hostname == "" {
		log.Printf("[ERROR]: could not get computer name: %v", err)
		report.Hostname = "UNKNOWN"
	}

	if debug {
		log.Println("[DEBUG]: report info ready")
	}

	if debug {
		log.Println("[DEBUG]: launching goroutines")
	}

	// These operations will be run using goroutines
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getComputerInfo(debug); err != nil {
			// Retry
			report.getComputerInfo(debug)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getOperatingSystemInfo(debug); err != nil {
			// Retry
			report.getOperatingSystemInfo(debug)
		}
	}()

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	if err := report.getOSInfo(debug); err != nil {
	// 		// Retry
	// 		report.getOSInfo(debug)
	// 	}
	// }()

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	if err := report.getMonitorsInfo(debug); err != nil {
	// 		// Retry
	// 		report.getMonitorsInfo(debug)
	// 	}
	// }()

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	if err := report.getPrintersInfo(debug); err != nil {
	// 		// Retry
	// 		report.getPrintersInfo(debug)
	// 	}
	// }()

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	if err := report.getSharesInfo(debug); err != nil {
	// 		// Retry
	// 		report.getSharesInfo(debug)
	// 	}
	// }()

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	if err := report.getAntivirusInfo(debug); err != nil {
	// 		// Retry
	// 		report.getAntivirusInfo(debug)
	// 	}
	// }()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getNetworkAdaptersInfo(debug); err != nil {
			// Retry
			report.getNetworkAdaptersInfo(debug)
		}
		// Get network adapter with default gateway and set its ip address and MAC as the report IP/MAC address
		for _, n := range report.NetworkAdapters {
			if n.DefaultGateway != "" {
				report.IP = n.Addresses
				report.MACAddress = n.MACAddress
				break
			}
		}
	}()

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	if err := report.getApplicationsInfo(debug); err != nil {
	// 		// Retry
	// 		report.getApplicationsInfo(debug)
	// 	}
	// }()

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	if err := report.getVNCInfo(debug); err != nil {
	// 		report.getVNCInfo(debug)
	// 	}
	// }()

	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	if err := report.getUpdateTaskInfo(debug); err != nil {
	// 		// Retry
	// 		report.getUpdateTaskInfo(debug)
	// 	}
	// }()

	wg.Wait()

	// // These tasks can affect previous tasks
	// if err := report.getSystemUpdateInfo(debug); err != nil {
	// 	// Retry
	// 	report.getSystemUpdateInfo(debug)
	// }

	// if err := report.getLogicalDisksInfo(debug); err != nil {
	// 	// Retry
	// 	report.getLogicalDisksInfo(debug)
	// }

	return &report, nil
}

func (r *Report) Print() {
	fmt.Printf("\n** 🕵  Agent *********************************************************************************************************\n")
	fmt.Printf("%-40s |  %s\n", "Computer Name", r.Hostname)
	// fmt.Printf("%-40s |  %s\n", "IP address", r.IP)
	fmt.Printf("%-40s |  %s\n", "Operating System", r.OS)

	r.logComputer()
	r.logOS()
	// r.logLogicalDisks()
	// r.logMonitors()
	// r.logPrinters()
	// r.logShares()
	// r.logAntivirus()
	// r.logSystemUpdate()
	r.logNetworkAdapters()
	// r.logApplications()
}
