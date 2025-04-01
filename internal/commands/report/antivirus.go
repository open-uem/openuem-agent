package report

import (
	"fmt"
)

type antivirusProduct struct {
	DisplayName              string
	ProductState             int
	PathToSignedProductExe   string
	PathToSignedReportingExe string
}

func (r *Report) logAntivirus() {
	fmt.Printf("\n** 🛡️ Antivirus *****************************************************************************************************\n")
	fmt.Printf("%-40s |  %v \n", "Antivirus installed", r.Antivirus.Name)
	fmt.Printf("%-40s |  %v \n", "Antivirus is active", r.Antivirus.IsActive)
	fmt.Printf("%-40s |  %t \n", "Antivirus database is updated", r.Antivirus.IsUpdated)
}
