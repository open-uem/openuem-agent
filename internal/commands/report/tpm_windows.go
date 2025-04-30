//go:build windows

package report

import (
	"context"
	"log"
)

func (r *Report) getTPMInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: TPM info has been requested")
	}
	if err := r.getTPMFromWMI(); err != nil {
		log.Printf("[ERROR]: could not get TPM information from WMI Win32_Tpm: %v", err)
		return err
	} else {
		log.Printf("[INFO]: TPM information has been retrieved from WMI Win32_Tpm")
	}
	return nil
}

func (r *Report) getTPMFromWMI() error {
	var tpmDst []tpmInfo

	namespace := `root/cimv2/Security/MicrosoftTpm`
	q := "SELECT IsActivated_InitialValue, IsEnabled_InitialValue, IsOwned_InitialValue, SpecVersion FROM Win32_Tpm"
	err := WMIQueryWithContext(context.Background(), q, &tpmDst, namespace)
	if err != nil {
		return err
	}

	// for _, v := range tpmDst {

	// }
	return nil
}
