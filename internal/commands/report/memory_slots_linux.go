// go:build linux

package report

import (
	"log"
	"os/exec"
	"regexp"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getMemorySlotsInfo(debug bool) error {
	r.MemorySlots = []openuem_nats.MemorySlot{}

	if debug {
		log.Println("[DEBUG]: memory slots info has been requested")
	}

	out, err := exec.Command("dmidecode", "--type", "17").Output()
	if err != nil {
		return err
	}

	reg := regexp.MustCompile(`(?:\tLocator: )(.*)`)
	matches := reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		mySlot := openuem_nats.MemorySlot{}
		mySlot.Slot = v[1]
		r.MemorySlots = append(r.MemorySlots, mySlot)
	}

	reg = regexp.MustCompile(`(?:\tType: )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.MemorySlots) > i {
			if v[1] == "Unknown" {
				continue
			}
			r.MemorySlots[i].MemoryType = v[1]
		}
	}

	reg = regexp.MustCompile(`(?:\tPart Number: )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		for i := range r.MemorySlots {
			if r.MemorySlots[i].MemoryType == "" || r.MemorySlots[i].PartNumber != "" {
				continue
			}
			r.MemorySlots[i].PartNumber = v[1]
			break
		}
	}

	reg = regexp.MustCompile(`(?:\tSerial Number: )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		for i := range r.MemorySlots {
			if r.MemorySlots[i].MemoryType == "" || r.MemorySlots[i].SerialNumber != "" {
				continue
			}
			r.MemorySlots[i].SerialNumber = v[1]
			break
		}
	}

	reg = regexp.MustCompile(`(?:\tSize: )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if v[1] == "No Module Installed" {
			r.MemorySlots[i].Size = ""
			continue
		}
		r.MemorySlots[i].Size = v[1]

	}

	reg = regexp.MustCompile(`(?:\tSpeed: )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		for i := range r.MemorySlots {
			if r.MemorySlots[i].MemoryType == "" || r.MemorySlots[i].Speed != "" {
				continue
			}
			r.MemorySlots[i].Speed = v[1]
			break
		}
	}

	reg = regexp.MustCompile(`(?:\tManufacturer: )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for _, v := range matches {
		for i := range r.MemorySlots {
			if r.MemorySlots[i].MemoryType == "" || r.MemorySlots[i].Manufacturer != "" {
				continue
			}
			r.MemorySlots[i].Manufacturer = v[1]
			break
		}
	}

	log.Printf("[INFO]: memory slots information has been retrieved from Linux dmidecode")
	return nil
}
