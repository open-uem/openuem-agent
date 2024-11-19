package report

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/doncicuto/openuem_nats"
)

func (r *Report) getMonitorsInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: monitors info has been requested")
	}

	// Get monitors information
	// Ref: https://learn.microsoft.com/en-us/windows/win32/wmicoreprov/wmimonitorid
	var monitorDst []struct {
		ManufacturerName []int32
		SerialNumberID   []int32
		UserFriendlyName []int32
	}

	r.Monitors = []openuem_nats.Monitor{}

	namespace := `root\wmi`
	qMonitors := "SELECT ManufacturerName, SerialNumberID, UserFriendlyName FROM WmiMonitorID"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qMonitors, &monitorDst, namespace)
	if err != nil {
		log.Printf("[ERROR]: could not get information from WMI WmiMonitorID: %v", err)
		return err
	}
	for _, v := range monitorDst {
		myMonitor := openuem_nats.Monitor{}
		myMonitor.Manufacturer = convertInt32ArrayToString(v.ManufacturerName)
		myMonitor.Model = convertInt32ArrayToString(v.UserFriendlyName)
		myMonitor.Serial = convertInt32ArrayToString(v.SerialNumberID)

		r.Monitors = append(r.Monitors, myMonitor)
	}
	log.Printf("[INFO]: monitors information has been retrieved from WMI WmiMonitorID")
	return nil
}

func (r *Report) logMonitors() {
	fmt.Printf("\n** 📺 Monitors ******************************************************************************************************\n")
	if len(r.Monitors) > 0 {
		for i, v := range r.Monitors {
			fmt.Printf("%-40s |  %s \n", "Manufacturer", v.Manufacturer)
			fmt.Printf("%-40s |  %s \n", "Model", v.Model)
			fmt.Printf("%-40s |  %s \n", "Serial number", v.Serial)
			if len(r.Monitors) > 1 && i+1 != len(r.Monitors) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No monitors found")
	}
}

func convertInt32ArrayToString(a []int32) string {
	str := ""
	for _, code := range a {
		str += string(rune(code))
	}
	return strings.Trim(str, "\x00")
}
