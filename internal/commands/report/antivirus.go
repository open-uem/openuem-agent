package report

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
)

type antivirusProduct struct {
	DisplayName              string
	ProductState             int
	PathToSignedProductExe   string
	PathToSignedReportingExe string
}

func (r *Report) getAntivirusInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: antivirus info has been requested")
	}
	if err := r.getAntivirusFromWMI(); err != nil {
		log.Printf("[ERROR]: could not get antivirus information from WMI AntiVirusProduct: %v", err)
		return err
	} else {
		log.Printf("[INFO]: antivirus information has been retrieved from WMI AntiVirusProduct")
	}
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

func (r *Report) logAntivirus() {
	fmt.Printf("\n** 🛡️ Antivirus *****************************************************************************************************\n")
	fmt.Printf("%-40s |  %v \n", "Antivirus installed", r.Antivirus.Name)
	fmt.Printf("%-40s |  %v \n", "Antivirus is active", r.Antivirus.IsActive)
	fmt.Printf("%-40s |  %t \n", "Antivirus database is updated", r.Antivirus.IsUpdated)
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
