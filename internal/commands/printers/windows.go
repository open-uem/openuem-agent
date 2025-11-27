//go:build windows

package printers

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/open-uem/openuem-agent/internal/commands/runtime"
)

func RemovePrinter(printerName string) error {
	args := []string{"Remove-Printer", "-Name", printerName}
	if err := exec.Command("powershell", args...).Run(); err != nil {
		return err
	}

	return nil
}

func SetDefaultPrinter(printerName string) error {
	// Fix: 129 a network printer may have this format \\IMLADRIS\PrinterName so we need to escape the backslash for Powershell
	printerName = strings.ReplaceAll(printerName, "\\", "\\\\")
	args := []string{"Invoke-CimMethod", "-InputObject", fmt.Sprintf(`(Get-CimInstance -Class Win32_Printer -Filter "Name='%s'")`, printerName), "-MethodName", "SetDefaultPrinter"}
	if err := runtime.RunAsUser("powershell", args); err != nil {
		return err
	}

	return nil
}
