//go:build windows

package report

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type antivirusProduct struct {
	DisplayName              string
	ProductState             int
	PathToSignedProductExe   string
	PathToSignedReportingExe string
}

// MSFT_MpComputerStatus is exposed by the Windows Defender WMI provider,
// which is also available on Windows Server (Desktop Experience and Core)
// where the root\SecurityCenter2 namespace does not exist.
type mpComputerStatus struct {
	AMServiceEnabled              bool
	AntivirusEnabled              bool
	RealTimeProtectionEnabled     bool
	AntivirusSignatureLastUpdated time.Time
}

func (r *Report) getAntivirusInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: antivirus info has been requested")
	}
	if err := r.getAntivirusFromWMI(); err != nil {
		log.Printf("[ERROR]: could not get antivirus information from WMI AntiVirusProduct: %v", err)
		if err := r.getAntivirusFromDefender(); err != nil {
			log.Printf("[ERROR]: could not get antivirus information from WMI MSFT_MpComputerStatus: %v", err)
			return err
		}
		log.Printf("[INFO]: antivirus information has been retrieved from WMI MSFT_MpComputerStatus")
		return nil
	}
	log.Printf("[INFO]: antivirus information has been retrieved from WMI AntiVirusProduct")
	return nil
}

func (r *Report) getAntivirusFromDefender() error {
	var dst []mpComputerStatus

	namespace := `root\Microsoft\Windows\Defender`
	q := "SELECT AMServiceEnabled, AntivirusEnabled, RealTimeProtectionEnabled, AntivirusSignatureLastUpdated FROM MSFT_MpComputerStatus"
	if err := WMIQueryWithContext(context.Background(), q, &dst, namespace); err != nil {
		return err
	}

	if len(dst) == 0 {
		return fmt.Errorf("MSFT_MpComputerStatus returned no rows")
	}

	status := dst[0]
	r.Antivirus.Name = "Microsoft Defender Antivirus"
	r.Antivirus.IsActive = status.AMServiceEnabled && status.AntivirusEnabled && status.RealTimeProtectionEnabled
	r.Antivirus.IsUpdated = time.Since(status.AntivirusSignatureLastUpdated) <= 7*24*time.Hour

	return nil
}

func (r *Report) getAntivirusFromWMI() error {
	// Get information about the antivirus
	// Ref: https://gist.github.com/whit3rabbit/02c1b8648635f3552483b7f9a0b459ea
	var avDst []antivirusProduct

	namespace := `root\SecurityCenter2`
	q := "SELECT displayName, productState, pathToSignedProductExe, pathToSignedReportingExe from AntiVirusProduct"
	err := WMIQueryWithContext(context.Background(), q, &avDst, namespace)
	if err != nil {
		return err
	}

	for _, v := range avDst {
		isActive := isAntivirusActive(v.ProductState)

		// Some antivirus vendors don't refresh WMI when its product is removed so we may well find several
		// vendors claiming that they're active and evend updated so we'll have to check if the executables
		// are installed but for Windows Defender
		if strings.TrimSpace(v.DisplayName) == "Windows Defender" {
			r.Antivirus.Name = strings.TrimSpace(v.DisplayName)
			r.Antivirus.IsActive = isActive
			r.Antivirus.IsUpdated = isSignatureDBUpdated(v.ProductState)

			if isActive {
				break
			}
		} else {
			if isAntivirusInstalled(v) {
				r.Antivirus.Name = v.DisplayName
				r.Antivirus.IsActive = isActive
				r.Antivirus.IsUpdated = isSignatureDBUpdated(v.ProductState)

				if isActive {
					break
				}
			}
		}
	}
	return nil
}

func isAntivirusActive(productState int) bool {
	switch productState & 0x0000F000 {
	case 0x0000:
		return false //Off
	case 0x1000:
		return true //On
	case 0x2000:
		return false //Snoozed
	case 0x3000:
		return false //Expired
	default:
		return false
	}
}

func isSignatureDBUpdated(productState int) bool {
	switch productState & 0x000000F0 {
	case 0x00:
		return true
	case 0x10:
		return false
	default:
		return false
	}
}

func isAntivirusInstalled(av antivirusProduct) bool {
	// WMI may show that an antivirus is still installed but that's because
	// the product uninstall didn't report to WMI so we check if executables
	// are there
	return fileExists(av.PathToSignedProductExe) || fileExists(av.PathToSignedReportingExe)
}

func fileExists(filename string) bool {
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}
