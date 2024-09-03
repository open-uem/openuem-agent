package agent

import (
	"fmt"
	"log"
	"strings"

	"github.com/yusufpapurcu/wmi"
)

type Computer struct {
	Manufacturer   string `json:"manufacturer,omitempty"`
	Model          string `json:"model,omitempty"`
	Serial         string `json:"serial,omitempty"`
	Processor      string `json:"processor,omitempty"`
	ProcessorArch  string `json:"processor_arch,omitempty"`
	ProcessorCores int64  `json:"processor_cores,omitempty"`
	Memory         uint64 `json:"memory,omitempty"`
}

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

func (a *Agent) getComputerInfo() {
	a.Edges.Computer = Computer{}
	if err := a.Edges.Computer.getComputerSystemInfo(); err != nil {
		log.Printf("[ERROR]: could not get information from WMI Win32_ComputerSystem: %v", err)
	} else {
		log.Printf("[INFO]: computer system info has been retrieved from WMI Win32_ComputerSystem")
	}

	if err := a.Edges.Computer.getSerialNumber(); err != nil {
		log.Printf("[ERROR]: could not get information from WMI Win32_Bios: %v", err)
	} else {
		log.Printf("[INFO]: serial number info has been retrieved from WMI Win32_Bios")
	}

	if err := a.Edges.Computer.getProcessorInfo(); err != nil {
		log.Printf("[ERROR]: could not get information from WMI Win32_Processor: %v", err)
	} else {
		log.Printf("[INFO]: processor info has been retrieved from WMI Win32_Processor")
	}
}

func (a *Agent) logComputer() {
	fmt.Printf("\n** 🖥️ Computer ******************************************************************************************************\n")
	fmt.Printf("%-40s |  %s \n", "Manufacturer", a.Edges.Computer.Manufacturer)
	fmt.Printf("%-40s |  %s \n", "Model", a.Edges.Computer.Model)
	fmt.Printf("%-40s |  %s \n", "Serial Number", a.Edges.Computer.Serial)
	fmt.Printf("%-40s |  %s \n", "Processor", a.Edges.Computer.Processor)
	fmt.Printf("%-40s |  %s \n", "Processor Architecture", a.Edges.Computer.ProcessorArch)
	fmt.Printf("%-40s |  %d \n", "Number of Cores", a.Edges.Computer.ProcessorCores)
	fmt.Printf("%-40s |  %d MB \n", "RAM Memory", a.Edges.Computer.Memory)
}

func (myComputer *Computer) getComputerSystemInfo() error {
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
	myComputer.Manufacturer = "Unknown"
	if v.Manufacturer != "System manufacturer" {
		myComputer.Manufacturer = strings.TrimSpace(v.Manufacturer)
	}
	myComputer.Model = "Unknown"
	if v.Model != "System Product Name" {
		myComputer.Model = strings.TrimSpace(v.Model)
	}
	myComputer.Memory = v.TotalPhysicalMemory / 1024 / 1024
	return nil
}

func (myComputer *Computer) getSerialNumber() error {
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
	myComputer.Serial = "Unknown"
	if v.SerialNumber != "System Serial Number" {
		myComputer.Serial = strings.TrimSpace(v.SerialNumber)
	}
	return nil
}

func (myComputer *Computer) getProcessorInfo() error {
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
	myComputer.Processor = strings.TrimSpace(v.Name)
	myComputer.ProcessorArch = getProcessorArch(v.Architecture)
	myComputer.ProcessorCores = int64(v.NumberOfCores)
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
