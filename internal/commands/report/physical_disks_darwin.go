//go:build darwin

package report

import (
	"encoding/json"
	"log"
	"os/exec"

	openuem_nats "github.com/open-uem/nats"
)

type SPSerialATADataTypes struct {
	Devices []SPSerialATADataTypeItems `json:"SPSerialATADataType"`
}

type SPSerialATADataTypeItems struct {
	Items []SPSerialATADataType `json:"_items"`
}

type SPSerialATADataType struct {
	Name   string `json:"bsd_name"`
	Serial string `json:"device_serial"`
	Model  string `json:"device_model"`
	Size   string `json:"size"`
}

func (r *Report) getPhysicalDisksInfo(debug bool) error {
	var devices SPSerialATADataTypes
	r.PhysicalDisks = []openuem_nats.PhysicalDisk{}

	log.Println("[DEBUG]: physical disk info retrieval started")

	out, err := exec.Command("system_profiler", "-json", "SPSerialATADataType").Output()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(out, &devices); err != nil {
		return err
	}

	if len(devices.Devices) == 0 || len(devices.Devices[0].Items) == 0 {
		return nil
	}

	for _, pd := range devices.Devices[0].Items {
		disk := openuem_nats.PhysicalDisk{
			DeviceID:     pd.Name,
			Model:        pd.Model,
			SerialNumber: pd.Serial,
			SizeInUnits:  pd.Size,
		}

		r.PhysicalDisks = append(r.PhysicalDisks, disk)
	}

	if debug {
		log.Println("[DEBUG]: physical disk info retrieval finished")
	}

	return nil
}
