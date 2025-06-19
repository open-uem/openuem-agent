//go:build darwin

package report

import (
	"encoding/json"
	"log"
	"os/exec"
	"runtime"
	"strconv"

	openuem_nats "github.com/open-uem/nats"
)

type SPMemoryDataTypeIntel struct {
	SPMemoryDataType []MemoryDataTypeIntel `json:"SPMemoryDataType"`
}

type MemoryDataTypeIntel struct {
	Name                string                `json:"_name"`
	GlobalECCState      string                `json:"global_ecc_state"`
	IsMemoryUpgradeable string                `json:"is_memory_upgradeable"`
	Items               []MemorySlotTypeIntel `json:"_items"`
}

type MemorySlotTypeIntel struct {
	Name         string `json:"_name"`
	Manufacturer string `json:"dimm_manufacturer"`
	PartNumber   string `json:"dimm_part_number"`
	SerialNumber string `json:"dimm_serial_number"`
	Size         string `json:"dimm_size"`
	Speed        string `json:"dimm_speed"`
	Status       string `json:"dimm_status"`
	MemoryType   string `json:"dimm_type"`
}

type SPMemoryDataTypeAppleSilicon struct {
	SPMemoryDataType []MemoryDataTypeAppleSilicon `json:"SPMemoryDataType"`
}

type MemoryDataTypeAppleSilicon struct {
	Manufacturer string `json:"dimm_manufacturer"`
	MemoryType   string `json:"dimm_type"`
	Size         string `json:"SPMemoryDataType"`
}

func (r *Report) getMemorySlotsInfo(debug bool) error {
	var memoryDataIntel SPMemoryDataTypeIntel
	var memoryDataTypeAppleSilicon SPMemoryDataTypeAppleSilicon

	r.MemorySlots = []openuem_nats.MemorySlot{}

	if debug {
		log.Println("[DEBUG]: memory slots info has been requested")
	}

	out, err := exec.Command("system_profiler", "-json", "SPMemoryDataType").Output()
	if err != nil {
		return err
	}

	if runtime.GOARCH == "amd64" {
		if err := json.Unmarshal(out, &memoryDataIntel); err != nil {
			return err
		}

		for _, data := range memoryDataIntel.SPMemoryDataType {
			for _, slot := range data.Items {
				mySlot := openuem_nats.MemorySlot{}
				mySlot.Slot = slot.Name
				mySlot.Manufacturer = slot.Manufacturer
				mySlot.MemoryType = slot.MemoryType
				mySlot.PartNumber = slot.PartNumber
				mySlot.SerialNumber = slot.SerialNumber
				mySlot.Size = slot.Size
				mySlot.Speed = slot.Speed
				r.MemorySlots = append(r.MemorySlots, mySlot)
			}
		}
	} else {
		if err := json.Unmarshal(out, &memoryDataTypeAppleSilicon); err != nil {
			return err
		}

		for index, slot := range memoryDataTypeAppleSilicon.SPMemoryDataType {
			mySlot := openuem_nats.MemorySlot{}
			mySlot.Slot = strconv.Itoa(index)
			mySlot.Manufacturer = slot.Manufacturer
			mySlot.MemoryType = slot.MemoryType
			mySlot.Size = slot.Size
			r.MemorySlots = append(r.MemorySlots, mySlot)
		}
	}

	log.Printf("[INFO]: memory slots information has been retrieved")
	return nil
}
