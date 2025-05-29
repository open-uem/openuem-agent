//go:build darwin

package report

import (
	"log"
	"os/exec"
	"strings"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getPrintersInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: printers info has been requested")
	}

	err := r.getPrintersFromLinux()
	if err != nil {
		log.Printf("[ERROR]: could not get printers information from Linux hwinfo: %v", err)
		return err
	} else {
		log.Printf("[INFO]: printers information has been retrieved from Linux hwinfo")
	}
	return nil
}

func (r *Report) getPrintersFromLinux() error {
	r.Printers = []openuem_nats.Printer{}

	getDefaultPrinter := "LANG=en_US.UTF-8 lpstat -d | awk '{print $4}'"
	out, err := exec.Command("bash", "-c", getDefaultPrinter).Output()
	if err != nil {
		return err
	}
	defaultPrinter := strings.TrimSpace(string(out))

	getPrinters := "LANG=en_US.UTF-8 lpstat -p | grep '^printer' | awk '{print $2}'"
	out, err = exec.Command("bash", "-c", getPrinters).Output()
	// out, err := exec.Command("hwinfo", "--printer").Output()
	if err != nil {
		return err
	}
	printers := strings.Split(string(out), "\n")

	getPorts := "LANG=en_US.UTF-8 lpstat -s | grep '^device' | awk '{print $4}'"
	out, err = exec.Command("bash", "-c", getPorts).Output()
	if err != nil {
		return err
	}
	ports := strings.Split(string(out), "\n")

	for index, printer := range printers {
		myPrinter := openuem_nats.Printer{}
		if printer != "" {
			myPrinter.Name = strings.TrimSpace(printer)
			myPrinter.IsDefault = myPrinter.Name == defaultPrinter
			if len(ports) > index {
				myPrinter.Port = ports[index]
				myPrinter.IsNetwork = strings.Contains(myPrinter.Port, "socket")
			} else {
				myPrinter.Port = "-"
			}
			r.Printers = append(r.Printers, myPrinter)
		}
	}

	// reg := regexp.MustCompile(`Model: "\s*(.*?)\s*"`)
	// matches := reg.FindAllStringSubmatch(string(out), -1)
	// for _, v := range matches {
	// 	myPrinter := openuem_nats.Printer{}
	// 	if v[1] == "" || v[1] == "0" {
	// 		myPrinter.Name = "Unknown"
	// 	} else {
	// 		myPrinter.Name = v[1]
	// 	}
	// 	myPrinter.Port = "-"
	// 	myPrinter.IsDefault = false
	// 	myPrinter.IsNetwork = false
	// 	r.Printers = append(r.Printers, myPrinter)
	// }

	return nil
}
