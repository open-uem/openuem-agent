package report

import (
	"fmt"
	"log"
	"strings"

	openuem_nats "github.com/open-uem/nats"
)

const (
	APPS32BITS = `SOFTWARE\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall`
	APPS       = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
)

func (r *Report) getApplicationsInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: applications info has been requested")
	}
	r.Applications = []openuem_nats.Application{}
	myApps, err := getApplications(debug)
	if err != nil {
		return err
	}
	for k, v := range myApps {
		app := openuem_nats.Application{}
		app.Name = strings.TrimSpace(k)
		app.Version = strings.TrimSpace(v.Version)
		app.InstallDate = strings.TrimSpace(v.InstallDate)
		app.Publisher = strings.TrimSpace(v.Publisher)
		r.Applications = append(r.Applications, app)
	}
	return nil
}

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
