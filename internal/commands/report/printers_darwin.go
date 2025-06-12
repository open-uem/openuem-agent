//go:build darwin

package report

import (
	"encoding/json"
	"log"
	"os/exec"
	"strings"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getPrintersInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: printers info has been requested")
	}

	err := r.getPrintersFromMac()
	if err != nil {
		log.Printf("[ERROR]: could not get printers information: %v", err)
		return err
	} else {
		log.Printf("[INFO]: printers information has been retrieved")
	}
	return nil
}

func (r *Report) getPrintersFromMac() error {
	var printerData SPPrintersDataType
	r.Printers = []openuem_nats.Printer{}

	out, err := exec.Command("system_profiler", "-json", "SPPrintersDataType").Output()
	if err != nil {
		return err
	}

	if strings.Contains(string(out), "empty") || strings.Contains(string(out), "no_info_found") {
		return nil
	}

	if err := json.Unmarshal(out, &printerData); err != nil {
		log.Printf("[ERROR]: could not unmarshal printers information: %s", out)
		return err
	}

	for _, printer := range printerData.SPPrintersDataType {
		myPrinter := openuem_nats.Printer{}
		myPrinter.Name = strings.TrimSpace(printer.Name)
		myPrinter.IsDefault = strings.Contains(printer.Default, "yes")
		myPrinter.Port = printer.URI
		myPrinter.IsNetwork = !strings.Contains(printer.PrintServer, "local")
		myPrinter.IsShared = strings.Contains(printer.Shared, "yes")
		r.Printers = append(r.Printers, myPrinter)
	}

	return nil
}

type SPPrintersDataType struct {
	SPPrintersDataType []PrinterDataType `json:"SPPrintersDataType"`
}

type PrinterDataType struct {
	Name            string `json:"_name"`
	CreationDate    string `json:"creationDate"`
	Default         string `json:"default"`
	DriverVersion   string `json:"driverversion"`
	FaxSupport      string `json:"Fax Support"`
	PPD             string `json:"ppd"`
	PPDFileVersion  string `json:"ppdfileversion"`
	PrinterSharing  string `json:"printersharing"`
	PrinterCommands string `json:"printercommands"`
	PrintServer     string `json:"printserver"`
	PSVersion       string `json:"psversion"`
	Scanner         string `json:"scanner"`
	Shared          string `json:"shared"`
	Status          string `json:"status"`
	URI             string `json:"uri"`
}
