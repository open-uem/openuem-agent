//go:build linux

package printers

import (
	"fmt"
	"os/exec"
)

func RemovePrinter(printerName string) error {
	removePrinter := fmt.Sprintf("lpadmin -x %s", printerName)
	return exec.Command("bash", "-c", removePrinter).Run()
}

func SetDefaultPrinter(printerName string) error {
	setDefaultPrinter := fmt.Sprintf("lpoptions -d %s", printerName)
	return exec.Command("bash", "-c", setDefaultPrinter).Run()
}
