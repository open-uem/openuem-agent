//go:build darwin

package report

import (
	"log"
	"strings"
)

func (r *Report) getComputerInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: computer system info has been requested")
	}
	if err := r.getComputerSystemInfo(); err != nil {
		log.Printf("[ERROR]: could not get information from SysInfo: %v", err)
		return err
	} else {
		log.Printf("[INFO]: computer system info has been retrieved from SysInfo")
	}

	if debug {
		log.Println("[DEBUG]: serial number has been requested")
	}
	if err := r.getSerialNumber(); err != nil {
		log.Printf("[ERROR]: could not get information from SysInfo: %v", err)
		return err
	} else {
		log.Printf("[INFO]: serial number info has been retrieved from SysInfo")
	}

	if debug {
		log.Println("[DEBUG]: processor info has been requested")
	}
	if err := r.getProcessorInfo(); err != nil {
		log.Printf("[ERROR]: could not get information from SysInfo: %v", err)
		return err
	} else {
		log.Printf("[INFO]: processor info has been retrieved from SysInfo")
	}
	return nil
}

func (r *Report) getComputerSystemInfo() error {

	r.Computer.Manufacturer = strings.TrimSpace("TODO")
	if r.Computer.Manufacturer == "" {
		r.Computer.Manufacturer = "Unknown"
	}
	r.Computer.Model = strings.TrimSpace("si.Product.Name")
	if r.Computer.Model == "System Product Name" {
		r.Computer.Model = "Unknown"
	}
	r.Computer.Memory = 0
	return nil
}

func (r *Report) getSerialNumber() error {
	r.Computer.Serial = "si.Product.Serial"
	if r.Computer.Serial == "System Serial Number" {
		r.Computer.Serial = "Unknown"
	}
	return nil
}

func (r *Report) getProcessorInfo() error {
	r.Computer.Processor = "si.CPU.Model"
	r.Computer.ProcessorArch = "si.Kernel.Architecture"
	r.Computer.ProcessorCores = 0
	return nil
}
