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
		if v[1] == "" || v[1] == "0" {
			mySlot.Slot = "Unknown"
		} else {
			mySlot.Slot = v[1]
		}
		r.MemorySlots = append(r.MemorySlots, mySlot)
	}

	reg = regexp.MustCompile(`(?:\tType: )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.MemorySlots) > i {
			r.MemorySlots[i].MemoryType = v[1]
		}
	}

	reg = regexp.MustCompile(`(?:\tPart Number: )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.MemorySlots) > i {
			r.MemorySlots[i].PartNumber = v[1]
		}
	}

	reg = regexp.MustCompile(`(?:\tSerial Number: )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.MemorySlots) > i {
			r.MemorySlots[i].SerialNumber = v[1]
		}
	}

	reg = regexp.MustCompile(`(?:\tSize: )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.MemorySlots) > i {
			r.MemorySlots[i].Size = v[1]
		}
	}

	reg = regexp.MustCompile(`(?:\tSpeed: )(.*)`)
	matches = reg.FindAllStringSubmatch(string(out), -1)
	for i, v := range matches {
		if len(r.MemorySlots) > i {
			r.MemorySlots[i].Speed = v[1]
		}
	}

	log.Printf("[INFO]: memory slots information has been retrieved from Linux dmidecode")
	return nil
}
