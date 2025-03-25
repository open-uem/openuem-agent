//go:build windows

package report

import (
	"context"
	"log"
)

func (r *Report) getSharesInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: shares info has been requested")
	}

	err := r.getSharesFromWMI()
	if err != nil {
		log.Printf("[ERROR]: could not get shares information from WMI Win32_Share: %v", err)
		return err
	} else {
		log.Printf("[INFO]: shares information has been retrieved from WMI Win32_Share")
	}
	return nil
}

func (r *Report) getSharesFromWMI() error {
	namespace := `root\cimv2`
	qShares := "SELECT Name, Path, Description FROM Win32_Share"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qShares, &r.Shares, namespace)
	if err != nil {
		return err
	}
	return nil
}
