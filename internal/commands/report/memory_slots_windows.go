//go:build windows

package report

import (
	"context"
	"log"
	"strconv"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getMemorySlotsInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: memory slots info has been requested")
	}

	// Get memory slots information
	// Ref: https://learn.microsoft.com/en-us/windows/win32/cimwin32prov/win32-physicalmemory
	var slotsDst []struct {
		DeviceLocator        string
		SerialNumber         string
		PartNumber           string
		Capacity             uint64
		ConfiguredClockSpeed uint32
	}

	r.MemorySlots = []openuem_nats.MemorySlot{}

	namespace := `root\wmi`
	qMonitors := "SELECT DeviceLocator, SerialNumber, PartNumber, Capacity, ConfiguredClockSpeed FROM Win32_PhysicalMemory"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qMonitors, &slotsDst, namespace)
	if err != nil {
		log.Printf("[ERROR]: could not get information from WMI WmiMonitorID: %v", err)
		return err
	}
	for _, v := range slotsDst {
		mySlot := openuem_nats.MemorySlot{}
		mySlot.Slot = v.DeviceLocator
		mySlot.PartNumber = v.PartNumber
		mySlot.SerialNumber = v.SerialNumber
		mySlot.Size = convertBytesToUnits(v.Capacity)
		mySlot.Speed = strconv.Itoa(int(v.ConfiguredClockSpeed))
		// TODO memory type -> SMBIOSMemoryType
		r.MemorySlots = append(r.MemorySlots, mySlot)
	}
	log.Printf("[INFO]: memory slots information has been retrieved from WMI WmiMonitorID")
	return nil
}
