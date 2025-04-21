//go:build windows

package report

import (
	"context"
	"fmt"
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
		SMBIOSMemoryType     uint16
		Manufacturer         string
	}

	r.MemorySlots = []openuem_nats.MemorySlot{}

	namespace := `root\cimv2`
	qMonitors := "SELECT DeviceLocator, SerialNumber, PartNumber, Capacity, ConfiguredClockSpeed, Manufacturer, SMBIOSMemoryType FROM Win32_PhysicalMemory"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qMonitors, &slotsDst, namespace)
	if err != nil {
		log.Printf("[ERROR]: could not get memory slots information from WMI Win32_PhysicalMemory: %v", err)
		return err
	}
	for _, v := range slotsDst {
		mySlot := openuem_nats.MemorySlot{}
		mySlot.Slot = v.DeviceLocator
		mySlot.PartNumber = v.PartNumber
		mySlot.SerialNumber = v.SerialNumber
		mySlot.Size = convertRAMBytesToUnits(v.Capacity)
		mySlot.Speed = fmt.Sprintf("%s MHz", strconv.Itoa(int(v.ConfiguredClockSpeed)))
		mySlot.Manufacturer = v.Manufacturer
		mySlot.MemoryType = convertUint16ToMemoryType(v.SMBIOSMemoryType)
		r.MemorySlots = append(r.MemorySlots, mySlot)
	}
	log.Printf("[INFO]: memory slots information has been retrieved from Win32_PhysicalMemory")
	return nil
}

func convertUint16ToMemoryType(memoryType uint16) string {

	switch memoryType {
	case 0:
		return "Unknown"
	case 1:
		return "Other"
	case 2:
		return "DRAM"
	case 3:
		return "Synchronous DRAM "
	case 4:
		return "Cache DRAM"
	case 5:
		return "EDO"
	case 6:
		return "EDRAM"
	case 7:
		return "VRAM"
	case 8:
		return "SRAM"
	case 9:
		return "RAM"
	case 10:
		return "ROM"
	case 11:
		return "Flash"
	case 12:
		return "EEPROM"
	case 13:
		return "FEPROM"
	case 14:
		return "EPROM"
	case 15:
		return "CDRAM"
	case 16:
		return "3DRAM"
	case 17:
		return "SDRAM"
	case 18:
		return "SGRAM"
	case 19:
		return "RDRAM"
	case 20:
		return "DDR"
	case 21:
		return "DDR2"
	case 22:
		return "DDR2 FB-DIMM"
	case 24:
		return "DDR3"
	case 26:
		return "DDR4"
	default:
		return "Unknown"
	}
}

func convertRAMBytesToUnits(size uint64) string {
	units := fmt.Sprintf("%d MB", size/1_048_576)
	if size/1_048_576 >= 1000 {
		units = fmt.Sprintf("%d GB", size/1_073_741_824)
	}
	if size/1_073_741_824 >= 1000 {
		units = fmt.Sprintf("%d TB", size/1_099_511_628_000)
	}
	if size/1_099_511_628_000 >= 1000 {
		units = fmt.Sprintf("%d PB", size/1_099_511_628_000)
	}
	return units
}
