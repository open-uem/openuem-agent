package report

import (
	"fmt"
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
