//go:build darwin

package report

import (
	"encoding/json"
	"log"
	"os/exec"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getMonitorsInfo(debug bool) error {
	var displaysData SPDisplaysDataType
	r.Monitors = []openuem_nats.Monitor{}

	if debug {
		log.Println("[DEBUG]: monitors info has been requested")
	}

	out, err := exec.Command("system_profiler", "-json", "SPDisplaysDataType").Output()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(out, &displaysData); err != nil {
		return err
	}

	for _, data := range displaysData.SPDisplaysDataType {
		for _, display := range data.DisplaysNDrvs {
			myMonitor := openuem_nats.Monitor{}
			myMonitor.Model = display.Name
			myMonitor.Manufacturer = display.VendorID
			myMonitor.Serial = display.SerialNumber
			r.Monitors = append(r.Monitors, myMonitor)
		}
	}

	log.Printf("[INFO]: monitors information has been retrieved")
	return nil
}

type SPDisplaysDataType struct {
	SPDisplaysDataType []DisplaDataType `json:"SPDisplaysDataType"`
}

type DisplaDataType struct {
	Name                 string            `json:"_name"`
	SPDisplaysDeviceID   string            `json:"spdisplays_device-id"`
	SPDisplaysRevisionID string            `json:"spdisplays_revision-id"`
	SPDisplaysVendorID   string            `json:"spdisplays_vendor-id"`
	SPDisplaysVRAM       string            `json:"spdisplays_vram"`
	SPPCIDeviceType      string            `json:"sppci_device_type"`
	DisplaysNDrvs        []DisplayNDrvType `json:"spdisplays_ndrvs"`
}

type DisplayNDrvType struct {
	Name            string `json:"_name"`
	ProductID       string `json:"_spdisplays_display-product-id"`
	SerialNumber    string `json:"_spdisplays_display-serial-number2"`
	VendorID        string `json:"_spdisplays_display-vendor-id"`
	DisplayID       string `json:"_spdisplays_displayID"`
	DisplayPath     string `json:"_spdisplays_displayPath"`
	RegID           string `json:"_spdisplays_displayRegID"`
	Pixels          string `json:"_spdisplays_pixels"`
	Resolution_     string `json:"_spdisplays_resolution"`
	ConnectionType  string `json:"spdisplays_connection_type"`
	Depth           string `json:"spdisplays_depth"`
	Main            string `json:"spdisplays_main"`
	Mirror          string `json:"spdisplays_mirror"`
	PixelResolution string `json:"spdisplays_pixelresolution"`
	Resolution      string `json:"spdisplays_resolution"`
}

// {
//   "SPDisplaysDataType" : [
//     {
//       "_name" : "spdisplays_display",
//       "spdisplays_device-id" : "0x1111",
//       "spdisplays_ndrvs" : [
//         {
//           "_name" : "Unknown Display",
//           "_spdisplays_display-product-id" : "717",
//           "_spdisplays_display-serial-number2" : "0",
//           "_spdisplays_display-vendor-id" : "756e6b6e",
//           "_spdisplays_displayID" : "5b81c5c0",
//           "_spdisplays_displayPath" : "IOService:/AppleACPIPlatformExpert/PCI0/AppleACPIPCI/S18@3/AppleBochVGAFB",
//           "_spdisplays_displayRegID" : "4a17",
//           "_spdisplays_pixels" : "1920 x 1080",
//           "_spdisplays_resolution" : "1920 x 1080 @ 85.00Hz",
//           "spdisplays_connection_type" : "spdisplays_internal",
//           "spdisplays_depth" : "CGSThirtytwoBitColor",
//           "spdisplays_main" : "spdisplays_yes",
//           "spdisplays_mirror" : "spdisplays_off",
//           "spdisplays_online" : "spdisplays_yes",
//           "spdisplays_pixelresolution" : "spdisplays_1080p",
//           "spdisplays_resolution" : "1920 x 1080 @ 85.00Hz"
//         }
//       ],
//       "spdisplays_revision-id" : "0x0002",
//       "spdisplays_vendor-id" : "0x1234",
//       "spdisplays_vram" : "7 MB",
//       "sppci_device_type" : "spdisplays_gpu"
//     }
//   ]
// }
