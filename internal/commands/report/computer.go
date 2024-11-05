package report

import (
	"fmt"
	"log"
	"strings"

	"github.com/yusufpapurcu/wmi"
)

type computerSystem struct {
	Manufacturer        string
	Model               string
	TotalPhysicalMemory uint64
}

type biosInfo struct {
	SerialNumber string
}

type processorInfo struct {
	Architecture  uint32
	Name          string
	NumberOfCores uint32
}

func (r *Report) getComputerInfo() error {
	if err := r.getComputerSystemInfo(); err != nil {
		log.Printf("[ERROR]: could not get information from WMI Win32_ComputerSystem: %v", err)
		return err
	} else {
		log.Printf("[INFO]: computer system info has been retrieved from WMI Win32_ComputerSystem")
	}

	if err := r.getSerialNumber(); err != nil {
		log.Printf("[ERROR]: could not get information from WMI Win32_Bios: %v", err)
		return err
	} else {
		log.Printf("[INFO]: serial number info has been retrieved from WMI Win32_Bios")
	}

	if err := r.getProcessorInfo(); err != nil {
		log.Printf("[ERROR]: could not get information from WMI Win32_Processor: %v", err)
		return err
	} else {
		log.Printf("[INFO]: processor info has been retrieved from WMI Win32_Processor")
	}
	return nil
}

func (r *Report) logComputer() {
	fmt.Printf("\n** 🖥️ Computer ******************************************************************************************************\n")
	fmt.Printf("%-40s |  %s \n", "Manufacturer", r.Computer.Manufacturer)
	fmt.Printf("%-40s |  %s \n", "Model", r.Computer.Model)
	fmt.Printf("%-40s |  %s \n", "Serial Number", r.Computer.Serial)
	fmt.Printf("%-40s |  %s \n", "Processor", r.Computer.Processor)
	fmt.Printf("%-40s |  %s \n", "Processor Architecture", r.Computer.ProcessorArch)
	fmt.Printf("%-40s |  %d \n", "Number of Cores", r.Computer.ProcessorCores)
	fmt.Printf("%-40s |  %d MB \n", "RAM Memory", r.Computer.Memory)
}

func (r *Report) getComputerSystemInfo() error {
	// Get computer system information
	// Ref: https://learn.microsoft.com/es-es/windows/win32/cimwin32prov/win32-computersystem
	computerDst := []computerSystem{}

	namespace := `root\cimv2`
	qComputer := "SELECT Manufacturer, Model, TotalPhysicalMemory FROM Win32_ComputerSystem"
	err := wmi.QueryNamespace(qComputer, &computerDst, namespace)
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
	err := wmi.QueryNamespace(qSerial, &serialDst, namespace)
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
	err := wmi.QueryNamespace(qProcessor, &processorDst, namespace)
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
