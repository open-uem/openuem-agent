//go:build windows

package report

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"

	openuem_nats "github.com/open-uem/nats"
)

type bitLockerStatus struct {
	ConversionStatus int8
	ProtectionStatus int8
	EncryptionMethod int8
}

type logicalDisk struct {
	DeviceID   string
	FreeSpace  uint64
	Size       uint64
	DriveType  uint32
	FileSystem string
	VolumeName string
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
			// DriveType 2 = Removable Disk (USB, flash drive, etc.)
			myDisk.IsRemovable = (v.DriveType == 2)

			myDisk.BitLockerStatus = getBitLockerStatus(myDisk.Label)

			if myDisk.BitLockerStatus == "Encrypted" {
				if debug {
					log.Println("[DEBUG]: bit locker recovery key has been requested for: ", myDisk.Label)
				}
				myDisk.BitLockerRecoveryKey = getBitLockerRecoveryKey(myDisk.Label)
			}

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
		log.Printf("[INFO]: no bitlocker result for drive %s got %d rows: %v", driveLetter, len(response), err)
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

// getBitLockerRecoveryKey retrieves the BitLocker numerical recovery password for a given drive.
// Uses PowerShell Get-BitLockerVolume which is available on Windows 10+ (Pro/Enterprise/Education).
// The service runs as LocalSystem which has sufficient privileges for this query.
// Only drives with ProtectionStatus == 1 (Encrypted) should be queried.
func getBitLockerRecoveryKey(driveLetter string) string {
	script := fmt.Sprintf(
		`(Get-BitLockerVolume -MountPoint '%s' -ErrorAction SilentlyContinue).KeyProtector | `+
			`Where-Object {$_.KeyProtectorType -eq 'RecoveryPassword'} | `+
			`Select-Object -First 1 -ExpandProperty RecoveryPassword`,
		driveLetter,
	)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		log.Printf("[INFO]: could not get BitLocker recovery key via PowerShell for %s: %v", driveLetter, err)
		return ""
	}

	key := strings.TrimSpace(string(out))
	if key != "" {
		log.Printf("[INFO]: BitLocker recovery key retrieved for drive %s", driveLetter)
	} else {
		log.Printf("[INFO]: no BitLocker recovery password found for drive %s", driveLetter)
	}
	return key
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
