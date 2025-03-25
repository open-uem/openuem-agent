package report

import (
	"fmt"
	"log"
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

func (r *Report) getComputerInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: computer system info has been requested")
	}
	if err := r.getComputerSystemInfo(); err != nil {
		log.Printf("[ERROR]: could not get information from WMI Win32_ComputerSystem: %v", err)
		return err
	} else {
		log.Printf("[INFO]: computer system info has been retrieved from WMI Win32_ComputerSystem")
	}

	if debug {
		log.Println("[DEBUG]: serial number has been requested")
	}
	if err := r.getSerialNumber(); err != nil {
		log.Printf("[ERROR]: could not get information from WMI Win32_Bios: %v", err)
		return err
	} else {
		log.Printf("[INFO]: serial number info has been retrieved from WMI Win32_Bios")
	}

	if debug {
		log.Println("[DEBUG]: processor info has been requested")
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
	fmt.Printf("\n** 🖥️  Computer ******************************************************************************************************\n")
	fmt.Printf("%-40s |  %s \n", "Manufacturer", r.Computer.Manufacturer)
	fmt.Printf("%-40s |  %s \n", "Model", r.Computer.Model)
	fmt.Printf("%-40s |  %s \n", "Serial Number", r.Computer.Serial)
	fmt.Printf("%-40s |  %s \n", "Processor", r.Computer.Processor)
	fmt.Printf("%-40s |  %s \n", "Processor Architecture", r.Computer.ProcessorArch)
	fmt.Printf("%-40s |  %d \n", "Number of Cores", r.Computer.ProcessorCores)
	fmt.Printf("%-40s |  %d MB \n", "RAM Memory", r.Computer.Memory)
}
