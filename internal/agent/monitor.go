package agent

import (
	"fmt"

	"github.com/doncicuto/openuem-agent/internal/log"
	"github.com/doncicuto/openuem-agent/internal/utils"
	"github.com/yusufpapurcu/wmi"
)

type Monitor struct {
	Manufacturer string `json:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty"`
	Serial       string `json:"serial,omitempty"`
}

func (a *Agent) getMonitorsInfo() {
	// Get monitors information
	// Ref: https://learn.microsoft.com/en-us/windows/win32/wmicoreprov/wmimonitorid
	var monitorDst []struct {
		ManufacturerName []int32
		SerialNumberID   []int32
		UserFriendlyName []int32
	}

	myMonitors := []Monitor{}

	namespace := `root\wmi`
	qMonitors := "SELECT ManufacturerName, SerialNumberID, UserFriendlyName FROM WmiMonitorID"
	err := wmi.QueryNamespace(qMonitors, &monitorDst, namespace)
	if err != nil {
		log.Logger.Printf("[ERROR]: could not get information from WMI WmiMonitorID: %v", err)
	}
	for _, v := range monitorDst {
		myMonitor := Monitor{}
		myMonitor.Manufacturer = utils.ConvertInt32ArrayToString(v.ManufacturerName)
		myMonitor.Model = utils.ConvertInt32ArrayToString(v.UserFriendlyName)
		myMonitor.Serial = utils.ConvertInt32ArrayToString(v.SerialNumberID)

		myMonitors = append(myMonitors, myMonitor)
	}
	a.Edges.Monitors = myMonitors
	log.Logger.Printf("[INFO]: monitors information has been retrieved from WMI WmiMonitorID")
}

func (a *Agent) logMonitors() {
	fmt.Printf("\n** 📺 Monitors ******************************************************************************************************\n")
	if len(a.Edges.Monitors) > 0 {
		for i, v := range a.Edges.Monitors {
			fmt.Printf("%-40s |  %s \n", "Manufacturer", v.Manufacturer)
			fmt.Printf("%-40s |  %s \n", "Model", v.Model)
			fmt.Printf("%-40s |  %s \n", "Serial number", v.Serial)
			if len(a.Edges.Monitors) > 1 && i+1 != len(a.Edges.Monitors) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No monitors found")
	}
}
