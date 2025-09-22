package report

import (
	"log"
)

func (r *Report) logPhysicalDisks() {
	log.Printf("\n** 💾 Physical Disks *************************************************************************************************\n")
	if len(r.LogicalDisks) > 0 {
		for i, myDisk := range r.PhysicalDisks {
			log.Printf("%-40s |  %s \n", "Disk ID", myDisk.DeviceID)
			log.Printf("%-40s |  %s \n", "Model", myDisk.Model)
			log.Printf("%-40s |  %s \n", "Serial Number", myDisk.SerialNumber)
			log.Printf("%-40s |  %s \n", "Size in units", myDisk.SizeInUnits)

			if len(r.PhysicalDisks) > 1 && i+1 != len(r.PhysicalDisks) {
				log.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
		return
	}

	log.Printf("%-40s\n", "No physical disks found")
}
