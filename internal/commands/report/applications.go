package report

import (
	"fmt"
)

const (
	APPS32BITS = `SOFTWARE\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall`
	APPS       = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
)

func (r *Report) logApplications() {
	fmt.Printf("\n** 📱 Software ******************************************************************************************************\n")
	if len(r.Applications) > 0 {
		for i, v := range r.Applications {
			fmt.Printf("%-40s |  %s \n", "Application", v.Name)
			fmt.Printf("%-40s |  %s \n", "Version", v.Version)
			fmt.Printf("%-40s |  %s \n", "Publisher", v.Publisher)
			fmt.Printf("%-40s |  %s \n", "Installation date", v.InstallDate)
			if len(r.Applications) > 1 && i+1 != len(r.Applications) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No applications found")
	}
}
