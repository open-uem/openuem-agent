package report

import (
	"fmt"
	"log"
	"strings"

	"github.com/doncicuto/openuem_nats"
	"github.com/yusufpapurcu/wmi"
)

type logicalDisk struct {
	DeviceID   string
	FreeSpace  uint64
	Size       uint64
	DriveType  uint32
	FileSystem string
	VolumeName string
}

type bitLockerStatus struct {
	ConversionStatus int8
	ProtectionStatus int8
	EncryptionMethod int8
}

func (r *Report) getLogicalDisksInfo() error {
	err := r.getLogicalDisksFromWMI()
	if err != nil {
		log.Printf("[ERROR]: could not get logical disks information from WMI Win32_LogicalDisk: %v", err)
		return err
	} else {
		log.Printf("[INFO]: logical disks information has been retrieved from WMI Win32_LogicalDisk")
	}
	return nil
}

func (r *Report) getLogicalDisksFromWMI() error {
	var disksDst []logicalDisk

	namespace := `root\cimv2`
	qLogicalDisk := "SELECT DeviceID, DriveType, FreeSpace, Size, FileSystem, VolumeName FROM Win32_LogicalDisk"
	err := wmi.QueryNamespace(qLogicalDisk, &disksDst, namespace)
	if err != nil {
		return err
	}
	for _, v := range disksDst {
		myDisk := openuem_nats.LogicalDisk{}

		if v.Size != 0 {
			myDisk.Label = strings.TrimSpace(v.DeviceID)
			myDisk.Usage = int8(100 - (v.FreeSpace * 100 / v.Size))
			myDisk.Filesystem = strings.TrimSpace(v.FileSystem)
			myDisk.VolumeName = strings.TrimSpace(v.VolumeName)

			myDisk.SizeInUnits = convertBytesToUnits(v.Size)
			myDisk.RemainingSpaceInUnits = convertBytesToUnits(v.FreeSpace)
			myDisk.BitLockerStatus = getBitLockerStatus(myDisk.Label)

			r.LogicalDisks = append(r.LogicalDisks, myDisk)
		}
	}
	return nil
}

func (r *Report) logLogicalDisks() {
	fmt.Printf("\n** 💾 Logical Disks *************************************************************************************************\n")
	if len(r.LogicalDisks) > 0 {
		for i, myDisk := range r.LogicalDisks {
			diskUsage := fmt.Sprintf("Disk %s usage", myDisk.Label)
			fmt.Printf("%-40s |  %d %% \n", diskUsage, myDisk.Usage)
			diskFreeSpace := fmt.Sprintf("Disk %s free space", myDisk.Label)
			fmt.Printf("%-40s |  %s \n", diskFreeSpace, myDisk.RemainingSpaceInUnits)
			diskSize := fmt.Sprintf("Disk %s size", myDisk.Label)
			fmt.Printf("%-40s |  %s \n", diskSize, myDisk.SizeInUnits)
			diskVolumeName := fmt.Sprintf("Disk %s volume name", myDisk.Label)
			fmt.Printf("%-40s |  %s \n", diskVolumeName, myDisk.VolumeName)
			diskFS := fmt.Sprintf("Disk %s filesystem", myDisk.Label)
			fmt.Printf("%-40s |  %s \n", diskFS, myDisk.Filesystem)
			fmt.Printf("%-40s |  %s \n", "Bitlocker Status", myDisk.BitLockerStatus)

			if len(r.LogicalDisks) > 1 && i+1 != len(r.LogicalDisks) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
		return
	}

	fmt.Printf("%-40s\n", "No logical disks found")
}

func getBitLockerStatus(driveLetter string) string {
	// This query would not be acceptable in general as it could lead to sql injection, but we're using a where condition using a
	// index value retrieved by WMI it's not user generated input
	namespace := `root\CIMV2\Security\MicrosoftVolumeEncryption`
	qBitLocker := fmt.Sprintf("SELECT ConversionStatus, ProtectionStatus, EncryptionMethod FROM Win32_EncryptableVolume WHERE DriveLetter = '%s'", driveLetter)
	response := []bitLockerStatus{}
	err := wmi.QueryNamespace(qBitLocker, &response, namespace)
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

func convertBytesToUnits(size uint64) string {
	units := fmt.Sprintf("%d MB", size/1_000_000)
	if size/1_000_000 >= 1000 {
		units = fmt.Sprintf("%d GB", size/1_000_000_000)
	}
	if size/1_000_000_000 >= 1000 {
		units = fmt.Sprintf("%d TB", size/1_000_000_000_000)
	}
	if size/1_000_000_000_000 >= 1000 {
		units = fmt.Sprintf("%d PB", size/1_000_000_000_000)
	}
	return units
}
