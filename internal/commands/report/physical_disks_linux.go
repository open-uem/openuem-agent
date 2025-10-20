//go:build linux

package report

import (
	"encoding/json"
	"log"
	"os/exec"

	openuem_nats "github.com/open-uem/nats"
)

type BlockDevice struct {
	Name   string `json:"name"`
	Serial string `json:"serial"`
	Model  string `json:"model"`
	Size   uint64 `json:"size"`
}

type BlockDevices struct {
	Devices []BlockDevice `json:"blockdevices"`
}

func (r *Report) getPhysicalDisksInfo(debug bool) error {
	var blockDevices BlockDevices
	r.PhysicalDisks = []openuem_nats.PhysicalDisk{}

	if debug {
		log.Println("[DEBUG]: physical disk info retrieval started")
	}

	out, err := exec.Command("lsblk", "--json", "--nodeps", "--bytes", "-o", "name,serial,model,size").Output()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(out, &blockDevices); err != nil {
		return err
	}

	for _, pd := range blockDevices.Devices {
		if pd.Size > 0 {
			disk := openuem_nats.PhysicalDisk{
				DeviceID:     pd.Name,
				Model:        pd.Model,
				SerialNumber: pd.Serial,
				SizeInUnits:  convertBytesToUnits(pd.Size),
			}

			r.PhysicalDisks = append(r.PhysicalDisks, disk)
		}
	}

	if debug {
		log.Println("[DEBUG]: physical disk info retrieval finished")
	}

	return nil
}
