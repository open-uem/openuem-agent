//go:build windows

package report

import (
	"context"
	"fmt"
	"strings"
)

func (r *Report) getComputerSystemInfo() error {
	// Get computer system information
	// Ref: https://learn.microsoft.com/es-es/windows/win32/cimwin32prov/win32-computersystem
	computerDst := []computerSystem{}

	namespace := `root\cimv2`
	qComputer := "SELECT Manufacturer, Model, TotalPhysicalMemory FROM Win32_ComputerSystem"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qComputer, &computerDst, namespace)
	if err != nil {
		return err
	}

	if len(computerDst) != 1 {
		return fmt.Errorf("got wrong computer system result set")
	}

	v := &computerDst[0]
	r.Computer.Manufacturer = "Unknown"
	if v.Manufacturer != "System manufacturer" {
		r.Computer.Manufacturer = strings.TrimSpace(v.Manufacturer)
	}
	r.Computer.Model = "Unknown"
	if v.Model != "System Product Name" {
		r.Computer.Model = strings.TrimSpace(v.Model)
	}
	r.Computer.Memory = v.TotalPhysicalMemory / 1024 / 1024
	return nil
}

func (r *Report) getSerialNumber() error {
	// Get SerialNumber from BIOSInfo
	// Ref: https://spurge.rentals/how-to-find-your-computers-bios-serial-number-a-guide-for-windows-macos-and-linux-users/
	var serialDst []biosInfo
	namespace := `root\cimv2`
	qSerial := "SELECT SerialNumber FROM Win32_Bios"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qSerial, &serialDst, namespace)
	if err != nil {
		return err
	}

	if len(serialDst) != 1 {
		return fmt.Errorf("got wrong bios result set")
	}

	v := &serialDst[0]
	r.Computer.Serial = "Unknown"
	if v.SerialNumber != "System Serial Number" {
		r.Computer.Serial = strings.TrimSpace(v.SerialNumber)
	}
	return nil
}

func (r *Report) getProcessorInfo() error {
	// Get Processor Info
	// Ref: https://devblogs.microsoft.com/scripting/use-powershell-and-wmi-to-get-processor-information/
	var processorDst []processorInfo
	namespace := `root\cimv2`
	qProcessor := "SELECT Architecture, Name, NumberOfCores FROM Win32_Processor"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qProcessor, &processorDst, namespace)
	if err != nil {
		return err
	}

	if len(processorDst) != 1 {
		return fmt.Errorf("got wrong processor result set")
	}

	v := processorDst[0]
	r.Computer.Processor = strings.TrimSpace(v.Name)
	r.Computer.ProcessorArch = getProcessorArch(v.Architecture)
	r.Computer.ProcessorCores = int64(v.NumberOfCores)
	return nil
}

func getProcessorArch(arch uint32) string {
	switch arch {
	case 0:
		return "x86"
	case 1:
		return "MIPS"
	case 2:
		return "Alfa"
	case 3:
		return "PowerPC"
	case 5:
		return "ARM"
	case 6:
		return "ia64"
	case 9:
		return "x64"
	case 12:
		return "ARM64"
	}
	return "Unknown"
}
