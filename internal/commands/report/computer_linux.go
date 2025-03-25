//go:build linux

package report

import (
	"strings"
	"syscall"

	"github.com/zcalusic/sysinfo"
)

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
