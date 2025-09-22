//go:build windows

package report

import (
	"context"
	"log"
	"strings"

	openuem_nats "github.com/open-uem/nats"
)

type physicalDisk struct {
	DeviceID     string
	Model        string
	Size         uint64
	SerialNumber string
}

func (r *Report) getPhysicalDisksFromWMI(debug bool) error {
	var disksDst []physicalDisk

	namespace := `root\cimv2`
	qDiskDrive := "SELECT DeviceID, Model, Size, SerialNumber FROM Win32_DiskDrive"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qDiskDrive, &disksDst, namespace)
	if err != nil {
		return err
	}
	for _, v := range disksDst {
		myDisk := openuem_nats.PhysicalDisk{}

		if v.Size != 0 {
			myDisk.DeviceID = strings.TrimSpace(v.DeviceID)
			if debug {
				log.Println("[DEBUG]: physical disk info started for: ", myDisk.DeviceID)
			}
			myDisk.Model = strings.TrimSpace(v.Model)
			myDisk.SerialNumber = strings.TrimSpace(v.SerialNumber)

			if debug {
				log.Println("[DEBUG]: bit locker status info has been requested for: ", myDisk.DeviceID)
			}

			r.PhysicalDisks = append(r.PhysicalDisks, myDisk)
			if debug {
				log.Println("[DEBUG]: physical disk info finished for: ", myDisk.DeviceID)
			}
		}
	}
	return nil
}

func (r *Report) getPhysicalDisksInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: physical disks info has been requested")
	}
	return r.getPhysicalDisksFromWMI(debug)
}
