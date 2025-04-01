//go:build windows

package report

import (
	"context"
	"fmt"
	"log"
	"strings"

	openuem_nats "github.com/open-uem/nats"
)

type bitLockerStatus struct {
	ConversionStatus int8
	ProtectionStatus int8
	EncryptionMethod int8
}

func (r *Report) getLogicalDisksFromWMI(debug bool) error {
	var disksDst []logicalDisk

	namespace := `root\cimv2`
	qLogicalDisk := "SELECT DeviceID, DriveType, FreeSpace, Size, FileSystem, VolumeName FROM Win32_LogicalDisk"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qLogicalDisk, &disksDst, namespace)
	if err != nil {
		return err
	}
	for _, v := range disksDst {
		myDisk := openuem_nats.LogicalDisk{}

		if v.Size != 0 {
			myDisk.Label = strings.TrimSpace(v.DeviceID)
			if debug {
				log.Println("[DEBUG]: logical disk info started for: ", myDisk.Label)
			}
			myDisk.Usage = int8(100 - (v.FreeSpace * 100 / v.Size))
			myDisk.Filesystem = strings.TrimSpace(v.FileSystem)
			myDisk.VolumeName = strings.TrimSpace(v.VolumeName)

			myDisk.SizeInUnits = convertBytesToUnits(v.Size)
			myDisk.RemainingSpaceInUnits = convertBytesToUnits(v.FreeSpace)

			if debug {
				log.Println("[DEBUG]: bit locker status info has been requested for: ", myDisk.Label)
			}

			// TODO - This query halts report if in sequence in go routine often works fine
			myDisk.BitLockerStatus = getBitLockerStatus(myDisk.Label)

			r.LogicalDisks = append(r.LogicalDisks, myDisk)
			if debug {
				log.Println("[DEBUG]: logical disk info finished for: ", myDisk.Label)
			}
		}
	}
	return nil
}

func (r *Report) getLogicalDisksInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: logical disks info has been requested")
	}
	err := r.getLogicalDisksFromWMI(debug)
	if err != nil {
		log.Printf("[ERROR]: could not get logical disks information from WMI Win32_LogicalDisk: %v", err)
		return err
	} else {
		log.Printf("[INFO]: logical disks information has been retrieved from WMI Win32_LogicalDisk")
	}
	return nil
}

func getBitLockerStatus(driveLetter string) string {
	// This query would not be acceptable in general as it could lead to sql injection, but we're using a where condition using a
	// index value retrieved by WMI it's not user generated input

	// This query is executed in powershell like this
	// Get-WmiObject("Win32_EncryptableVolume") -Namespace "root\CIMV2\Security\MicrosoftVolumeEncryption" -ComputerName Win11WSL | where DriveLetter -eq "C:" | Format-List DriveLetter, ConversionStatus, ProtectionStatus, EncryptionMethod
	namespace := `root\CIMV2\Security\MicrosoftVolumeEncryption`
	qBitLocker := fmt.Sprintf("SELECT ConversionStatus, ProtectionStatus, EncryptionMethod FROM Win32_EncryptableVolume WHERE DriveLetter = '%s'", driveLetter)
	response := []bitLockerStatus{}

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qBitLocker, &response, namespace)
	if err != nil {
		log.Printf("[ERROR]: could not get bitlocker status from WMI Win32_EncryptableVolume: %v", err)
		return "Unknown"
	}

	if len(response) != 1 {
		log.Printf("[WARN]: no bitlocker result for drive %s got %d rows: %v", driveLetter, len(response), err)
		return "Unknown"
	}

	log.Printf("[INFO]: could get bitlocker status from WMI Win32_EncryptableVolume for driver %s", driveLetter)
	switch response[0].ProtectionStatus {
	case 0:
		return "Unencrypted"
	case 1:
		return "Encrypted"
	default:
		return "Unknown"
	}
}
