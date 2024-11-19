package report

import (
	"context"
	"fmt"
	"log"
)

// TODO logon date with WMI shows as protected with *****
// Another approach: https://gist.github.com/talatham/5772146
func (r *Report) getLoggedOnUserInfo(debug bool) {
	if debug {
		log.Println("[DEBUG]: logged on users info has been requested")
	}

	err := r.getLoggedOnUserFromWMI()
	if err != nil {
		log.Printf("[ERROR]: could not get logged on user information from WMI Win32_NetworkLoginProfile: %v", err)
	} else {
		log.Printf("[INFO]: logged on user information has been retrieved from WMI Win32_NetworkLoginProfile")
	}
}

func (r *Report) getLoggedOnUserFromWMI() error {
	namespace := `root\cimv2`
	q := "SELECT Name, LastLogon from Win32_NetworkLoginProfile"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, q, &r.LoggedOnUsers, namespace)
	if err != nil {
		return err
	}
	return nil
}

func (r *Report) logLoggedOnUsers() {
	fmt.Printf("\n** 👥 Logged On Users **********************************************************************************************\n")
	if len(r.LoggedOnUsers) > 0 {
		for i, v := range r.LoggedOnUsers {
			fmt.Printf("%-40s |  %v \n", "Name", v.Name)
			fmt.Printf("%-40s |  %s \n", "Last logon", v.LastLogon)
			if len(r.Printers) > 1 && i+1 != len(r.Printers) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No logged on users found")
	}
}
