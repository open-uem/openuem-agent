package agent

import (
	"fmt"
	"log"

	"github.com/yusufpapurcu/wmi"
)

type Share struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path,omitempty"`
}

func (a *Agent) getSharesInfo() {
	var err error
	a.Edges.Shares, err = getSharesFromWMI()
	if err != nil {
		log.Printf("[ERROR]: could not get shares information from WMI Win32_Share: %v", err)
	} else {
		log.Printf("[INFO]: shares information has been retrieved from WMI Win32_Share")
	}
}

func (a *Agent) logShares() {
	fmt.Printf("\n** 📤 Shares ********************************************************************************************************\n")
	if len(a.Edges.Shares) > 0 {
		for i, v := range a.Edges.Shares {
			fmt.Printf("%-40s |  %s \n", "Name", v.Name)
			fmt.Printf("%-40s |  %s \n", "Description", v.Description)
			fmt.Printf("%-40s |  %s \n", "Path", v.Path)
			if len(a.Edges.Shares) > 1 && i+1 != len(a.Edges.Shares) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No shares found")
	}
}

func getSharesFromWMI() ([]Share, error) {
	myShares := []Share{}

	namespace := `root\cimv2`
	qShares := "SELECT Name, Path, Description FROM Win32_Share"
	err := wmi.QueryNamespace(qShares, &myShares, namespace)
	if err != nil {
		return nil, err
	}
	return myShares, nil
}
