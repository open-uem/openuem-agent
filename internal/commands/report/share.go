package report

import (
	"fmt"
	"log"

	"github.com/yusufpapurcu/wmi"
)

func (r *Report) getSharesInfo() error {
	err := r.getSharesFromWMI()
	if err != nil {
		log.Printf("[ERROR]: could not get shares information from WMI Win32_Share: %v", err)
		return err
	} else {
		log.Printf("[INFO]: shares information has been retrieved from WMI Win32_Share")
	}
	return nil
}

func (r *Report) logShares() {
	fmt.Printf("\n** 📤 Shares ********************************************************************************************************\n")
	if len(r.Shares) > 0 {
		for i, v := range r.Shares {
			fmt.Printf("%-40s |  %s \n", "Name", v.Name)
			fmt.Printf("%-40s |  %s \n", "Description", v.Description)
			fmt.Printf("%-40s |  %s \n", "Path", v.Path)
			if len(r.Shares) > 1 && i+1 != len(r.Shares) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No shares found")
	}
}

func (r *Report) getSharesFromWMI() error {
	namespace := `root\cimv2`
	qShares := "SELECT Name, Path, Description FROM Win32_Share"
	err := wmi.QueryNamespace(qShares, &r.Shares, namespace)
	if err != nil {
		return err
	}
	return nil
}
