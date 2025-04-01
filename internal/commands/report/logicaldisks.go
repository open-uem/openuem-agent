package report

import (
	"fmt"
)

type logicalDisk struct {
	DeviceID   string
	FreeSpace  uint64
	Size       uint64
	DriveType  uint32
	FileSystem string
	VolumeName string
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

func convertLinuxBytesToUnits(size uint64) string {
	units := fmt.Sprintf("%d MB", size/1_048_576)
	if size/1_048_576 >= 1000 {
		units = fmt.Sprintf("%d GB", size/1_073_741_824)
	}
	if size/1_073_741_824 >= 1000 {
		units = fmt.Sprintf("%d TB", size/1_099_511_628_000)
	}
	if size/1_099_511_628_000 >= 1000 {
		units = fmt.Sprintf("%d PB", size/1_099_511_628_000)
	}
	return units
}
