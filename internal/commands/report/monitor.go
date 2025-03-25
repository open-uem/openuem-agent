package report

import (
	"fmt"
)

func (r *Report) logMonitors() {
	fmt.Printf("\n** 📺 Monitors ******************************************************************************************************\n")
	if len(r.Monitors) > 0 {
		for i, v := range r.Monitors {
			fmt.Printf("%-40s |  %s \n", "Manufacturer", v.Manufacturer)
			fmt.Printf("%-40s |  %s \n", "Model", v.Model)
			fmt.Printf("%-40s |  %s \n", "Serial number", v.Serial)
			if len(r.Monitors) > 1 && i+1 != len(r.Monitors) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No monitors found")
	}
}
