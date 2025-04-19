//go:build windows

package report

import (
	"context"
	"log"
	"strconv"
	"strings"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getMonitorsInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: monitors info has been requested")
	}

	// Get monitors information
	// Ref: https://learn.microsoft.com/en-us/windows/win32/wmicoreprov/wmimonitorid
	var monitorDst []struct {
		ManufacturerName  []int32
		SerialNumberID    []int32
		UserFriendlyName  []int32
		WeekOfManufacture uint8
		YearOfManufacture uint16
	}

	r.Monitors = []openuem_nats.Monitor{}

	namespace := `root\wmi`
	qMonitors := "SELECT ManufacturerName, SerialNumberID, UserFriendlyName, WeekOfManufacture, YearOfManufacture FROM WmiMonitorID"

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
		myMonitor.WeekOfManufacture = strconv.Itoa(int(v.WeekOfManufacture))
		myMonitor.YearOfManufacture = strconv.Itoa(int(v.YearOfManufacture))

		r.Monitors = append(r.Monitors, myMonitor)
	}
	log.Printf("[INFO]: monitors information has been retrieved from WMI WmiMonitorID")
	return nil
}

func convertInt32ArrayToString(a []int32) string {
	str := ""
	for _, code := range a {
		str += string(rune(code))
	}
	return strings.Trim(str, "\x00")
}
