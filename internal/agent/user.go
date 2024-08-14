package agent

import (
	"fmt"
	"time"

	"github.com/doncicuto/openuem-agent/internal/log"
	"github.com/yusufpapurcu/wmi"
)

type LoggedOnUser struct {
	Name      string    `json:"name,omitempty"`
	LastLogon time.Time `json:"last_logon,omitempty"`
}

// TODO logon date with WMI shows as protected with *****
// Another approach: https://gist.github.com/talatham/5772146
func (a *Agent) getLoggedOnUserInfo() {
	var err error

	a.Edges.LoggedOnUsers, err = getLoggedOnUserFromWMI()
	if err != nil {
		log.Logger.Printf("[ERROR]: could not get logged on user information from WMI Win32_NetworkLoginProfile: %v", err)
	} else {
		log.Logger.Printf("[INFO]: logged on user information has been retrieved from WMI Win32_NetworkLoginProfile")
	}
}

func getLoggedOnUserFromWMI() ([]LoggedOnUser, error) {
	// Get information about the antivirus
	// Ref: https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32-networkloginprofile
	var userDst []LoggedOnUser

	namespace := `root\cimv2`
	q := "SELECT Name, LastLogon from Win32_NetworkLoginProfile"
	err := wmi.QueryNamespace(q, &userDst, namespace)
	if err != nil {
		return nil, err
	}

	return userDst, nil
}

func (a *Agent) logLoggedOnUsers() {
	fmt.Printf("\n** 👥 Logged On Users **********************************************************************************************\n")
	if len(a.Edges.LoggedOnUsers) > 0 {
		for i, v := range a.Edges.LoggedOnUsers {
			fmt.Printf("%-40s |  %v \n", "Name", v.Name)
			fmt.Printf("%-40s |  %s \n", "Last logon", v.LastLogon)
			if len(a.Edges.Printers) > 1 && i+1 != len(a.Edges.Printers) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No logged on users found")
	}
}
