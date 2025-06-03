// go:build darwin

package report

import (
	"encoding/json"
	"log"
	"os/exec"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getMemorySlotsInfo(debug bool) error {
	var memoryData SPMemoryDataType

	r.MemorySlots = []openuem_nats.MemorySlot{}

	if debug {
		log.Println("[DEBUG]: memory slots info has been requested")
	}

	out, err := exec.Command("system_profiler", "-json", "SPMemoryDataType").Output()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(out, &memoryData); err != nil {
		return err
	}

	for _, data := range memoryData.SPMemoryDataType {
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

	log.Printf("[INFO]: memory slots information has been retrieved")
	return nil
}

type SPMemoryDataType struct {
	SPMemoryDataType []MemoryDataType `json:"SPMemoryDataType"`
}

type MemoryDataType struct {
	Name                string           `json:"_name"`
	GlobalECCState      string           `json:"global_ecc_state"`
	IsMemoryUpgradeable string           `json:"is_memory_upgradeable"`
	Items               []MemorySlotType `json:"_items"`
}

type MemorySlotType struct {
	Name         string `json:"_name"`
	Manufacturer string `json:"dimm_manufacturer"`
	PartNumber   string `json:"dimm_part_number"`
	SerialNumber string `json:"dimm_serial_number"`
	Size         string `json:"dimm_size"`
	Speed        string `json:"dimm_speed"`
	Status       string `json:"dimm_status"`
	MemoryType   string `json:"dimm_type"`
}
