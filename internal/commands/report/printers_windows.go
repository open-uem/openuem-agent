//go:build windows

package report

import (
	"context"
	"log"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getPrintersInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: printers info has been requested")
	}

	err := r.getPrintersFromWMI()
	if err != nil {
		log.Printf("[ERROR]: could not get printers information from WMI Win32_Printer: %v", err)
		return err
	} else {
		log.Printf("[INFO]: printers information has been retrieved from WMI Win32_Printer")
	}
	return nil
}

func (r *Report) getPrintersFromWMI() error {
	// Get Printers information
	// Ref: https://learn.microsoft.com/en-us/windows/win32/wmicoreprov/wmimonitorid
	var printersDst []struct {
		Default  bool
		Name     string
		Network  bool
		PortName string
		printerStatus
	}

	r.Printers = []openuem_nats.Printer{}
	namespace := `root\cimv2`
	qPrinters := "SELECT Name, Default, PortName, PrinterStatus, Network FROM Win32_Printer"

	ctx := context.Background()

	if r.OperatingSystem.Username != "" {
		err := WMIQueryWithContextAsUser(ctx, qPrinters, &printersDst, namespace, r.OperatingSystem.Username)
		if err != nil {
			return err
		}
	} else {
		err := WMIQueryWithContext(ctx, qPrinters, &printersDst, namespace)
		if err != nil {
			return err
		}
	}

	for _, v := range printersDst {
		myPrinter := openuem_nats.Printer{}
		myPrinter.Name = v.Name
		myPrinter.Port = v.PortName
		myPrinter.IsDefault = v.Default
		myPrinter.IsNetwork = v.Network
		r.Printers = append(r.Printers, myPrinter)
	}
	return nil
}
