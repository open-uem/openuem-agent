package report

import (
	"fmt"
	"log"

	"github.com/doncicuto/openuem_nats"
	"github.com/yusufpapurcu/wmi"
)

type printerStatus uint16

const (
	PRINTER_STATUS_OTHER printerStatus = iota + 1
	PRINTER_STATUS_UNKNOWN
	PRINTER_STATUS_IDLE
	PRINTER_STATUS_PRINTING
	PRINTER_STATUS_WARMING_UP
	PRINTER_STATUS_STOPPED_PRINTING
	PRINTER_STATUS_OFFLINE
	PRINTER_STATUS_PAUSED
	PRINTER_STATUS_ERROR
	PRINTER_STATUS_BUSY
	PRINTER_STATUS_NOT_AVAILABLE
	PRINTER_STATUS_WAITING
	PRINTER_STATUS_PROCESSING
	PRINTER_STATUS_INITIALIZATION
	PRINTER_STATUS_POWER_SAVE
	PRINTER_STATUS_PENDING_DELETION
	PRINTER_STATUS_IO_ACTIVE
	PRINTER_STATUS_MANUAL_FEED
)

func (r *Report) getPrintersInfo() error {
	err := r.getPrintersFromWMI()
	if err != nil {
		log.Printf("[ERROR]: could not get printers information from WMI Win32_Printer: %v", err)
		return err
	} else {
		log.Printf("[INFO]: printers information has been retrieved from WMI Win32_Printer")
	}
	return nil
}

func (r *Report) logPrinters() {
	fmt.Printf("\n** 🖨️ Printers ******************************************************************************************************\n")
	if len(r.Printers) > 0 {
		for i, v := range r.Printers {
			fmt.Printf("%-40s |  %s \n", "Name", v.Name)
			fmt.Printf("%-40s |  %s \n", "Port", v.Port)
			fmt.Printf("%-40s |  %t \n", "Is Default Printer", v.IsDefault)
			fmt.Printf("%-40s |  %t \n", "Is Network Printer", v.IsNetwork)
			if len(r.Printers) > 1 && i+1 != len(r.Printers) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No printers found")
	}
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
	err := wmi.QueryNamespace(qPrinters, &printersDst, namespace)
	if err != nil {
		return err
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
