package report

import (
	"fmt"
)

func (r *Report) logSystemUpdate() {
	fmt.Printf("\n** 🔄 Updates *******************************************************************************************************\n")
	fmt.Printf("%-40s |  %s \n", "Automatic Updates status", r.SystemUpdate.Status)
	if r.SystemUpdate.LastInstall.IsZero() {
		fmt.Printf("%-40s |  %s \n", "Last updates installation date", "Unknown")
	} else {
		fmt.Printf("%-40s |  %v \n", "Last updates installation date", r.SystemUpdate.LastInstall)
	}
	if r.SystemUpdate.LastSearch.IsZero() {
		fmt.Printf("%-40s |  %s \n", "Last updates search date", "Unknown")
	} else {
		fmt.Printf("%-40s |  %v \n", "Last updates search date", r.SystemUpdate.LastSearch)
	}
	fmt.Printf("%-40s |  %t \n", "Pending updates", r.SystemUpdate.PendingUpdates)
	if len(r.PendingUpdates) > 0 {
		fmt.Printf("%-40s |  %s \n", "Pending updates detail", PendingUpdatesDetailString(r.PendingUpdates))
	}
}
