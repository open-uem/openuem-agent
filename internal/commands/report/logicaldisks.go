package report

import (
	"fmt"
)

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
