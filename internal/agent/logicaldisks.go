package agent

import (
	"fmt"
	"strings"

	"github.com/doncicuto/openuem-agent/internal/log"
	"github.com/doncicuto/openuem-agent/internal/utils"
	"github.com/yusufpapurcu/wmi"
)

type LogicalDisk struct {
	Label                 string `json:"label,omitempty"`
	Usage                 int8   `json:"usage,omitempty"`
	Filesystem            string `json:"filesystem,omitempty"`
	SizeInUnits           string `json:"size_in_units,omitempty"`
	RemainingSpaceInUnits string `json:"remaining_space_in_units,omitempty"`
	VolumeName            string `json:"volume_name,omitempty"`
}

type logicalDisk struct {
	DeviceID   string
	FreeSpace  uint64
	Size       uint64
	DriveType  uint32
	FileSystem string
	VolumeName string
}

func (a *Agent) getLogicalDisksInfo() {
	var disksDst []logicalDisk
	myDisks := []LogicalDisk{}

	namespace := `root\cimv2`
	qLogicalDisk := "SELECT DeviceID, DriveType, FreeSpace, Size, FileSystem, VolumeName FROM Win32_LogicalDisk"
	err := wmi.QueryNamespace(qLogicalDisk, &disksDst, namespace)
	if err != nil {
		log.Logger.Printf("[ERROR]: could not get logical disks information from WMI Win32_LogicalDisk: %v", err)
	}
	for _, v := range disksDst {
		myDisk := LogicalDisk{}

		if v.Size != 0 {
			myDisk.Label = strings.TrimSpace(v.DeviceID)
			myDisk.Usage = int8(100 - (v.FreeSpace * 100 / v.Size))
			myDisk.Filesystem = strings.TrimSpace(v.FileSystem)
			myDisk.VolumeName = strings.TrimSpace(v.VolumeName)

			myDisk.SizeInUnits = utils.ConvertBytesToUnits(v.Size)
			myDisk.RemainingSpaceInUnits = utils.ConvertBytesToUnits(v.FreeSpace)

			myDisks = append(myDisks, myDisk)
		}
	}
	a.Edges.LogicalDisks = myDisks
	log.Logger.Printf("[INFO]: logical disks information has been retrieved from WMI Win32_LogicalDisk")
}

func (a *Agent) logLogicalDisks() {
	fmt.Printf("\n** 💾 Logical Disks *************************************************************************************************\n")
	if len(a.Edges.LogicalDisks) > 0 {
		for i, myDisk := range a.Edges.LogicalDisks {
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

			if len(a.Edges.LogicalDisks) > 1 && i+1 != len(a.Edges.LogicalDisks) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No logical disks found")
	}

}
