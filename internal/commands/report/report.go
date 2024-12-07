package report

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/doncicuto/comshim"
	"github.com/doncicuto/openuem_nats"
	"github.com/doncicuto/openuem_utils"
	"golang.org/x/sys/windows"
	"gopkg.in/ini.v1"
)

type Report struct {
	openuem_nats.AgentReport
}

func RunReport(agentId string, debug bool, vncProxyPort, sftpPort string) (*Report, error) {
	var wg sync.WaitGroup
	var err error

	if debug {
		log.Println("[DEBUG]: preparing com")
	}
	// Prepare COM
	if err := comshim.Add(1); err != nil {
		log.Printf("[ERROR]: run report could not add initial thread, %v", err)
		return nil, err
	}
	defer func() {
		if err := comshim.Done(); err != nil {
			log.Printf("[ERROR]: run report got en error in comshim Done, %v", err)
		}
	}()

	if debug {
		log.Println("[DEBUG]: com prepared")
	}

	if debug {
		log.Println("[DEBUG]: preparing report info")
	}

	report := Report{}
	report.AgentID = agentId
	report.OS = "windows"
	report.SFTPPort = sftpPort
	report.VNCProxyPort = vncProxyPort
	report.CertificateReady = isCertificateReady()

	// Check if a restart is still required
	// Get conf file
	configFile := openuem_utils.GetConfigFile()

	// Open ini file
	cfg, err := ini.Load(configFile)
	if err != nil {
		return nil, err
	}

	key, err := cfg.Section("Agent").GetKey("RestartRequired")
	if err != nil {
		log.Println("[ERROR]: could not read RestartRequired from INI")
		return nil, err
	}

	report.RestartRequired, err = key.Bool()
	if err != nil {
		log.Println("[ERROR]: could not parse RestartRequired")
		return nil, err
	}

	report.Release = openuem_nats.Release{
		Version: VERSION,
		Arch:    runtime.GOARCH,
		Os:      runtime.GOOS,
		Channel: CHANNEL,
	}
	report.ExecutionTime = time.Now()

	report.Hostname, err = windows.ComputerName()
	if err != nil {
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getOSInfo(debug); err != nil {
			// Retry
			report.getOSInfo(debug)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getMonitorsInfo(debug); err != nil {
			// Retry
			report.getMonitorsInfo(debug)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getPrintersInfo(debug); err != nil {
			// Retry
			report.getPrintersInfo(debug)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getSharesInfo(debug); err != nil {
			// Retry
			report.getSharesInfo(debug)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getAntivirusInfo(debug); err != nil {
			// Retry
			report.getAntivirusInfo(debug)
		}
	}()

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

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getApplicationsInfo(debug); err != nil {
			// Retry
			report.getApplicationsInfo(debug)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getVNCInfo(debug); err != nil {
			report.getVNCInfo(debug)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getUpdateTaskInfo(debug); err != nil {
			// Retry
			report.getUpdateTaskInfo(debug)
		}
	}()

	wg.Wait()

	// These tasks can affect previous tasks
	if err := report.getSystemUpdateInfo(debug); err != nil {
		// Retry
		report.getSystemUpdateInfo(debug)
	}

	if err := report.getLogicalDisksInfo(debug); err != nil {
		// Retry
		report.getLogicalDisksInfo(debug)
	}

	return &report, nil
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

func isCertificateReady() bool {
	wd, err := openuem_utils.GetWd()
	if err != nil {
		log.Println("[ERROR]: could not get working directory")
		return false
	}

	certPath := filepath.Join(wd, "certificates", "server.cer")
	_, err = os.Stat(certPath)
	if err != nil {
		return false
	}

	keyPath := filepath.Join(wd, "certificates", "server.key")
	_, err = os.Stat(keyPath)
	if err != nil {
		return false
	}

	return true
}
