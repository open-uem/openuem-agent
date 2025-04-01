package report

import (
	"fmt"
)

func (r *Report) logShares() {
	fmt.Printf("\n** 📤 Shares ********************************************************************************************************\n")
	if len(r.Shares) > 0 {
		for i, v := range r.Shares {
			fmt.Printf("%-40s |  %s \n", "Name", v.Name)
			fmt.Printf("%-40s |  %s \n", "Description", v.Description)
			fmt.Printf("%-40s |  %s \n", "Path", v.Path)
			if len(r.Shares) > 1 && i+1 != len(r.Shares) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No shares found")
	}
}
