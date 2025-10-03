//go:build linux

package report

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	openuem_nats "github.com/open-uem/nats"
	"github.com/zcalusic/sysinfo"
)

func RunReport(agentId string, enabled, debug bool, vncProxyPort, sftpPort, ipAddress string, sftpDisabled, remoteAssistanceDisabled bool, tenantID string, siteID string) (*Report, error) {
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
	report.SftpServiceDisabled = sftpDisabled
	report.RemoteAssistanceDisabled = remoteAssistanceDisabled
	report.Tenant = tenantID
	report.Site = siteID

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
		if err := report.getMemorySlotsInfo(debug); err != nil {
			// Retry
			report.getMemorySlotsInfo(debug)
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
		if err := report.getSharesInfo(); err != nil {
			// Retry
			report.getSharesInfo()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getAntivirusInfo(); err != nil {
			// Retry
			report.getAntivirusInfo()
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
				if n.Addresses == "" {
					report.IP = ipAddress
				} else {
					report.IP = n.Addresses
				}
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
		if err := report.getRemoteDesktopInfo(debug); err != nil {
			report.getRemoteDesktopInfo(debug)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		report.hasRustDesk(debug)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getUpdateTaskInfo(debug); err != nil {
			// Retry
			report.getUpdateTaskInfo(debug)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := report.getPhysicalDisksInfo(debug); err != nil {
			log.Printf("[ERROR]: could not get physical disks information: %v", err)
		} else {
			log.Printf("[INFO]: physical disks information has been retrieved")
		}
	}()

	wg.Wait()

	// These tasks can affect previous tasks
	if err := report.getSystemUpdateInfo(); err != nil {
		// Retry
		report.getSystemUpdateInfo()
	}

	if err := report.getLogicalDisksInfo(debug); err != nil {
		// Retry
		report.getLogicalDisksInfo(debug)
	}

	return &report, nil
}

func isCertificateReady() bool {
	wd := "/etc/openuem-agent"

	certPath := filepath.Join(wd, "certificates", "server.cer")
	_, err := os.Stat(certPath)
	if err != nil {
		return false
	}

	keyPath := filepath.Join(wd, "certificates", "server.key")
	_, err = os.Stat(keyPath)
	return err == nil
}
