//go:build linux

package report

import (
	"log"
	"strings"
	"syscall"

	"github.com/zcalusic/sysinfo"
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
	var si sysinfo.SysInfo

	si.GetSysInfo()

	r.Computer.Manufacturer = strings.TrimSpace(si.Product.Vendor)
	if r.Computer.Manufacturer == "" {
		r.Computer.Manufacturer = "Unknown"
	}
	r.Computer.Model = strings.TrimSpace(si.Product.Name)
	if r.Computer.Model == "System Product Name" {
		r.Computer.Model = "Unknown"
	}
	r.Computer.Memory = sysTotalMemory()
	return nil
}

func (r *Report) getSerialNumber() error {
	var si sysinfo.SysInfo

	si.GetSysInfo()

	r.Computer.Serial = si.Product.Serial
	if r.Computer.Serial == "System Serial Number" {
		r.Computer.Serial = "Unknown"
	}
	return nil
}

func (r *Report) getProcessorInfo() error {
	var si sysinfo.SysInfo

	si.GetSysInfo()

	r.Computer.Processor = si.CPU.Model
	r.Computer.ProcessorArch = si.Kernel.Architecture
	r.Computer.ProcessorCores = int64(si.CPU.Cores)
	return nil
}

func sysTotalMemory() uint64 {
	in := &syscall.Sysinfo_t{}
	err := syscall.Sysinfo(in)
	if err != nil {
		return 0
	}

	// If this is a 32-bit system, then these fields are
	// uint32 instead of uint64.
	// So we always convert to uint64 to match signature.
	return (uint64(in.Totalram) * uint64(in.Unit) / 1000 / 1000)
}
