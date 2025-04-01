package report

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	openuem_nats "github.com/open-uem/nats"
	openuem_utils "github.com/open-uem/utils"
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

func isCertificateReady() bool {
	wd, err := openuem_utils.GetWd()
	if err != nil {
		log.Println("[ERROR]: could not get working directory")
		return false
	}

	certPath := filepath.Join(wd, "certificates", "server.cer")
	_, err = os.Stat(certPath)
	if err != nil {
		return false
	}

	keyPath := filepath.Join(wd, "certificates", "server.key")
	_, err = os.Stat(keyPath)
	return err == nil
}
