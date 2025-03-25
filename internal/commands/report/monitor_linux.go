//go:build linux

package report

import (
	"log"
	"os/exec"
	"regexp"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getMonitorsInfo(debug bool) error {
	r.Monitors = []openuem_nats.Monitor{}

	if debug {
		log.Println("[DEBUG]: monitors info has been requested")
	}

	out, err := exec.Command("hwinfo", "--monitor").Output()
	if err != nil {
		return err
	}

	reg := regexp.MustCompile(`Serial ID: "\s*(.*?)\s*"`)
	matches := reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		myMonitor := openuem_nats.Monitor{}
		if v[1] == "" || v[1] == "0" {
			myMonitor.Serial = "Unknown"
		} else {
			myMonitor.Serial = v[1]
		}
		r.Monitors = append(r.Monitors, myMonitor)
	}

	reg = regexp.MustCompile(`Model: "\s*(.*?)\s*"`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.Monitors) > i {
			r.Monitors[i].Model = v[1]
		}
	}

	reg = regexp.MustCompile(`Vendor: \s*(.*?)\s* `)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.Monitors) > i {
			r.Monitors[i].Manufacturer = v[1]
		}
	}

	log.Printf("[INFO]: monitors information has been retrieved from Linux hwinfo")
	return nil
}
