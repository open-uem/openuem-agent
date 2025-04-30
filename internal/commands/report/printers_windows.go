//go:build windows

package report

import (
	"log"
	"regexp"
	"strconv"
	"strings"

	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func (r *Report) getPrintersInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: printers info has been requested")
	}

	err := r.getPrintersFromPowershell()
	if err != nil {
		log.Printf("[ERROR]: could not get printers information from WMI Win32_Printer: %v", err)
		return err
	} else {
		log.Printf("[INFO]: printers information has been retrieved from WMI Win32_Printer")
	}
	return nil
}

func (r *Report) getPrintersFromPowershell() error {
	r.Printers = []openuem_nats.Printer{}

	out, err := runtime.RunAsUserWithOutput("powershell", []string{"gwmi", "Win32_Printer | Select Name, Default, PortName, Network, Shared"})
	if err != nil {
		log.Printf("[ERROR]: could not run powershell to get printers, reason: %v\n", err)
	}

	reg := regexp.MustCompile(`(?:Name     : )(.*)`)
	matches := reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		myPrinter := openuem_nats.Printer{}
		myPrinter.Name = strings.TrimSuffix(v[1], "\r")
		r.Printers = append(r.Printers, myPrinter)
	}

	reg = regexp.MustCompile(`(?:Default  : )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.Printers) > i {
			isDefault, err := strconv.ParseBool(strings.TrimSuffix(v[1], "\r"))
			if err == nil {
				r.Printers[i].IsDefault = isDefault
			}
		}
	}

	reg = regexp.MustCompile(`(?:PortName : )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.Printers) > i {
			r.Printers[i].Port = strings.TrimSuffix(v[1], "\r")
		}
	}

	reg = regexp.MustCompile(`(?:Network  : )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.Printers) > i {
			isNetwork, err := strconv.ParseBool(strings.TrimSuffix(v[1], "\r"))
			if err == nil {
				r.Printers[i].IsNetwork = isNetwork
			}
		}
	}

	reg = regexp.MustCompile(`(?:Shared   : )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.Printers) > i {
			isShared, err := strconv.ParseBool(strings.TrimSuffix(v[1], "\r"))
			if err == nil {
				r.Printers[i].IsShared = isShared
			}
		}
	}

	return nil
}

// func (r *Report) getPrintersFromWMI() error {
// 	// Get Printers information
// 	// Ref: https://learn.microsoft.com/en-us/windows/win32/wmicoreprov/wmimonitorid
// 	var printersDst []struct {
// 		Default  bool
// 		Name     string
// 		Network  bool
// 		PortName string
// 		printerStatus
// 	}

// 	r.Printers = []openuem_nats.Printer{}
// 	namespace := `root\cimv2`
// 	qPrinters := "SELECT Name, Default, PortName, PrinterStatus, Network FROM Win32_Printer"

// 	ctx := context.Background()

// 	if r.OperatingSystem.Username != "" {
// 		err := WMIQueryWithContextAsUser(ctx, qPrinters, &printersDst, namespace, r.OperatingSystem.Username)
// 		if err != nil {
// 			return err
// 		}
// 		log.Println("[INFO]: printers info has been retrieved for logged in user")
// 	} else {
// 		err := WMIQueryWithContext(ctx, qPrinters, &printersDst, namespace)
// 		if err != nil {
// 			return err
// 		}
// 		log.Println("[INFO]: printers info has been retrieved for nt system/authority")
// 	}

// 	for _, v := range printersDst {
// 		myPrinter := openuem_nats.Printer{}
// 		myPrinter.Name = v.Name
// 		myPrinter.Port = v.PortName
// 		myPrinter.IsDefault = v.Default
// 		myPrinter.IsNetwork = v.Network
// 		r.Printers = append(r.Printers, myPrinter)
// 	}
// 	return nil
// }
