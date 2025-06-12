//go:build darwin

package report

import (
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"

	openuem_nats "github.com/open-uem/nats"
)

func (r *Report) getLogicalDisksInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: logical disks info has been requested")
	}
	err := r.getLogicalDisksFromMacOS(debug)
	if err != nil {
		log.Printf("[ERROR]: could not get logical disks information: %v", err)
		return err
	} else {
		log.Println("[INFO]: logical disks information has been retrieved")
	}
	return nil
}

type dfInfo struct {
	used      uint64
	available uint64
}

func (r *Report) getLogicalDisksFromMacOS(debug bool) error {
	if debug {
		log.Println("[DEBUG]: logical disks info has been requested")
	}

	diskUsage := make(map[string]dfInfo)

	// let's execute mount to find current df usage
	dfCommand := `df | grep -v Filesystem | grep -v devfs | grep -v map | grep -v AssetsV2 | awk '{print $1,$3,$4}'`
	out, err := exec.Command("bash", "-c", dfCommand).Output()
	if err != nil {
		return err
	}

	dfOutput := string(out)
	for df := range strings.SplitSeq(dfOutput, "\n") {
		dfData := strings.Split(df, " ")
		if dfData[0] != "" {
			info := dfInfo{}
			info.used, err = strconv.ParseUint(dfData[1], 10, 64)
			if err != nil {
				continue
			}
			info.available, err = strconv.ParseUint(dfData[2], 10, 64)
			if err != nil {
				continue
			}
			diskUsage[dfData[0]] = info
		}
	}

	// let's execute mount to find current mount points
	mountCommand := `mount | grep -v devfs | grep -v autofs | grep -v "AssetsV2" | awk '{print $1","$3","$4}'`
	out, err = exec.Command("bash", "-c", mountCommand).Output()
	if err != nil {
		return err
	}
	mountOutput := string(out)

	for mountPoint := range strings.SplitSeq(mountOutput, "\n") {
		mountData := strings.Split(mountPoint, ",")
		if mountData[0] != "" {
			du, ok := diskUsage[mountData[0]]
			if !ok {
				continue
			}

			myDisk := openuem_nats.LogicalDisk{}
			myDisk.Label = mountData[1]
			myDisk.Filesystem = strings.TrimSuffix(strings.TrimPrefix(mountData[2], "("), ",")
			myDisk.SizeInUnits = convertBytesToUnits(du.available + du.used)
			myDisk.RemainingSpaceInUnits = convertBytesToUnits(du.available)
			myDisk.Usage = int8(du.used * 100 / (du.available + du.used))
			myDisk.BitLockerStatus = "Unsupported"
			myDisk.VolumeName = mountData[0]
			r.LogicalDisks = append(r.LogicalDisks, myDisk)
		}
	}

	return nil
}

func convertBytesToUnits(size uint64) string {
	units := fmt.Sprintf("%d MB", size*512/1_048_576)
	if size*512/1_048_576 >= 1000 {
		units = fmt.Sprintf("%d GB", size*512/1_073_741_824)
	}
	if size*512/1_073_741_824 >= 1000 {
		units = fmt.Sprintf("%d TB", size*512/1_099_511_628_000)
	}
	if size*512/1_099_511_628_000 >= 1000 {
		units = fmt.Sprintf("%d PB", size*512/1_099_511_628_000)
	}
	return units
}
