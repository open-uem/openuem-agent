//go:build linux

package report

import (
	"log"
	"strings"

	"github.com/moby/sys/mountinfo"
	openuem_nats "github.com/open-uem/nats"
	"golang.org/x/sys/unix"
)

func (r *Report) getLogicalDisksInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: logical disks info has been requested")
	}
	err := r.getLogicalDisksFromLinux(debug)
	if err != nil {
		log.Printf("[ERROR]: could not get logical disks information from mountinfo: %v", err)
		return err
	} else {
		log.Println("[INFO]: logical disks information has been retrieved from mountinfo")
	}
	return nil
}

func (r *Report) getLogicalDisksFromLinux(debug bool) error {

	// Filter out squashfs, tmpfs...
	filter := mountinfo.FSTypeFilter("ext4", "ext3", "ext2", "vfat", "btrfs", "xfs", "zfs")

	// Get Linux mount points
	mounts, err := mountinfo.GetMounts(filter)
	if err != nil {
		return err
	}

	for _, m := range mounts {
		var stat unix.Statfs_t
		if !strings.Contains(m.Mountpoint, "snap") {
			if debug {
				log.Println("[DEBUG]: logical disk info started for: ", m.Mountpoint)
			}
			if err := unix.Statfs(m.Mountpoint, &stat); err != nil {
				log.Printf("[ERROR]: could not get information for mountpoint %s, reason: %v", m.Mountpoint, err)
				continue
			}
			myDisk := openuem_nats.LogicalDisk{}
			myDisk.Label = m.Mountpoint
			myDisk.Filesystem = m.FSType

			totalSize := stat.Blocks * uint64(stat.Bsize)
			availableSize := stat.Bavail * uint64(stat.Bsize)
			myDisk.SizeInUnits = convertLinuxBytesToUnits(totalSize)
			myDisk.RemainingSpaceInUnits = convertLinuxBytesToUnits(availableSize)
			myDisk.Usage = int8(100 - (availableSize * 100 / totalSize))
			myDisk.BitLockerStatus = "Unsupported"
			myDisk.VolumeName = m.Source
			r.LogicalDisks = append(r.LogicalDisks, myDisk)
		}
	}

	return nil
}
