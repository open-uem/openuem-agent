//go:build linux

package report

import (
	"log"
	"os/exec"
	"regexp"

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

	out, err := exec.Command("hwinfo", "--printer").Output()
	if err != nil {
		return err
	}

	reg := regexp.MustCompile(`Model: "\s*(.*?)\s*"`)
	matches := reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		myPrinter := openuem_nats.Printer{}
		if v[1] == "" || v[1] == "0" {
			myPrinter.Name = "Unknown"
		} else {
			myPrinter.Name = v[1]
		}
		myPrinter.Port = "-"
		myPrinter.IsDefault = false
		myPrinter.IsNetwork = false
		r.Printers = append(r.Printers, myPrinter)
	}

	return nil
}
