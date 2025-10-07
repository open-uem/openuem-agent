package report

import (
	"fmt"

	openuem_nats "github.com/open-uem/nats"
)

type Report struct {
	openuem_nats.AgentReport
}

func (r *Report) logOS() {
	fmt.Printf("\n** 📔 Operating System **********************************************************************************************\n")
	fmt.Printf("%-40s |  %s \n", "OS Version", r.OperatingSystem.Version)
	fmt.Printf("%-40s |  %s \n", "OS Description", r.OperatingSystem.Description)
	fmt.Printf("%-40s |  %s \n", "Install Date", r.OperatingSystem.InstallDate)
	fmt.Printf("%-40s |  %s \n", "OS Edition", r.OperatingSystem.Edition)
	fmt.Printf("%-40s |  %s \n", "OS Architecture", r.OperatingSystem.Arch)
	fmt.Printf("%-40s |  %s \n", "Last Boot Up Time", r.OperatingSystem.LastBootUpTime)
	fmt.Printf("%-40s |  %s \n", "User Name", r.OperatingSystem.Username)
}

func (r *Report) Print() {
	fmt.Printf("\n** 🕵  Agent *********************************************************************************************************\n")
	fmt.Printf("%-40s |  %s\n", "Computer Name", r.Hostname)
	fmt.Printf("%-40s |  %s\n", "IP address", r.IP)
	fmt.Printf("%-40s |  %s\n", "Operating System", r.OS)

	r.logComputer()
	r.logOS()
	r.logPhysicalDisks()
	r.logLogicalDisks()
	r.logMonitors()
	r.logPrinters()
	r.logShares()
	r.logAntivirus()
	r.logSystemUpdate()
	r.logNetworkAdapters()
	r.logApplications()
}
