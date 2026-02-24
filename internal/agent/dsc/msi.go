//go:build windows

package dsc

import (
	"errors"
	"fmt"
	"strings"
)

func InstallMSIPackage(path string, extraArguments string, logPath string) (string, string, error) {

	if path == "" {
		return "", "", errors.New("path cannot be empty")
	}

	// Installation path
	arguments := fmt.Sprintf("/I \"%s\" ", path)

	// Extra args
	arguments += extraArguments

	// Optional log
	if logPath != "" {
		arguments += fmt.Sprintf(" /log \"%s\"", logPath)
	}

	// Default arguments
	arguments += " /quiet /norestart"

	// Execute command
	command := fmt.Sprintf("(Start-Process msiexec.exe -Wait -Passthru -ArgumentList '%s').ExitCode", arguments)

	stdout, stderr, err := RunTaskWithLowPriority(command)
	if err != nil {
		return "", "", err
	}

	if stderr != "" {
		return "", stderr, nil
	}

	if strings.TrimSpace(stdout) == "0" {
		return "The action completed successfully", "", nil
	}

	return "", getMSIErrorCodeDescription(strings.TrimSpace(stdout)), nil
}

func UninstallMSIPackage(path string, extraArguments string, logPath string) (string, string, error) {

	if path == "" {
		return "", "", errors.New("path cannot be empty")
	}

	// Installation path
	arguments := fmt.Sprintf("/X \"%s\" ", path)

	// Extra args
	arguments += extraArguments

	// Optional log
	if logPath != "" {
		arguments += fmt.Sprintf(" /log \"%s\"", logPath)
	}

	// Default arguments
	arguments += " /quiet /norestart"

	// Execute command
	command := fmt.Sprintf("(Start-Process msiexec.exe -Wait -Passthru -ArgumentList '%s').ExitCode", arguments)

	stdout, stderr, err := RunTaskWithLowPriority(command)
	if err != nil {
		return "", "", err
	}

	if stderr != "" {
		return "", stderr, nil
	}

	if strings.TrimSpace(stdout) == "0" {
		return "The action completed successfully", "", nil
	}

	return "", getMSIErrorCodeDescription(strings.TrimSpace(stdout)), nil
}

func getMSIErrorCodeDescription(code string) string {
	switch code {
	case "13":
		return "The data is invalid"
	case "87":
		return "One of the parameters was invalid"
	case "120":
		return "This value is returned when a custom action attempts to call a function that can't be called from custom actions. The function returns the value ERROR_CALL_NOT_IMPLEMENTED"
	case "1259":
		return "If Windows Installer determines a product might be incompatible with the current operating system, it displays a dialog box informing the user and asking whether to try to install anyway. This error code is returned if the user chooses not to try the installation"
	case "1601":
		return "The Windows Installer service couldn't be accessed. Contact your support personnel to verify that the Windows Installer service is properly registered"
	case "1602":
		return "The user canceled installation"
	case "1603":
		return "A fatal error occurred during installation"
	case "1604":
		return "Installation suspended, incomplete"
	case "1605":
		return "This action is only valid for products that are currently installed"
	case "1606":
		return "The feature identifier isn't registered"
	case "1607":
		return "The component identifier isn't registered"
	case "1608":
		return "This is an unknown property"
	case "1609":
		return "The handle is in an invalid state"
	case "1610":
		return "The configuration data for this product is corrupt. Contact your support personnel"
	case "1611":
		return "The component qualifier not present"
	case "1612":
		return "The installation source for this product isn't available. Verify that the source exists and that you can access it"
	case "1613":
		return "This installation package can't be installed by the Windows Installer service. You must install a Windows service pack that contains a newer version of the Windows Installer service"
	case "1614":
		return "The product is uninstalled"
	case "1615":
		return "The SQL query syntax is invalid or unsupported"
	case "1616":
		return "The record field does not exist"
	case "1618":
		return "Another installation is already in progress. Complete that installation before proceeding with this install. For information about the mutex, see _MSIExecute Mutex"
	case "1619":
		return "This installation package couldn't be opened. Verify that the package exists and is accessible, or contact the application vendor to verify that this is a valid Windows Installer package"
	case "1620":
		return "This installation package couldn't be opened. Contact the application vendor to verify that this is a valid Windows Installer package"
	case "1621":
		return "There was an error starting the Windows Installer service user interface. Contact your support personnel"
	case "1622":
		return "There was an error opening installation log file. Verify that the specified log file location exists and is writable"
	case "1623":
		return "This language of this installation package isn't supported by your system"
	case "1624":
		return "There was an error applying transforms. Verify that the specified transform paths are valid"
	case "1625":
		return "This installation is forbidden by system policy. Contact your system administrator"
	case "1626":
		return "The function couldn't be executed"
	case "1627":
		return "The function failed during execution"
	case "1628":
		return "An invalid or unknown table was specified"
	case "1629":
		return "The data supplied is the wrong type"
	case "1630":
		return "Data of this type isn't supported"
	case "1631":
		return "The Windows Installer service failed to start. Contact your support personnel"
	case "1632":
		return "The Temp folder is either full or inaccessible. Verify that the Temp folder exists and that you can write to it"
	case "1633":
		return "This installation package isn't supported on this platform. Contact your application vendor"
	case "1634":
		return "Component isn't used on this machine"
	case "1635":
		return "This patch package couldn't be opened. Verify that the patch package exists and is accessible, or contact the application vendor to verify that this is a valid Windows Installer patch package"
	case "1636":
		return "This patch package couldn't be opened. Contact the application vendor to verify that this is a valid Windows Installer patch package"
	case "1637":
		return "This patch package can't be processed by the Windows Installer service. You must install a Windows service pack that contains a newer version of the Windows Installer service"
	case "1638":
		return "Another version of this product is already installed. Installation of this version can't continue. To configure or remove the existing version of this product, use Add/Remove Programs in Control Panel"
	case "1639":
		return "Invalid command line argument. Consult the Windows Installer SDK for detailed command-line help"
	case "1640":
		return "The current user isn't permitted to perform installations from a client session of a server running the Terminal Server role service"
	case "1641":
		return "The installer has initiated a restart. This message indicates success"
	case "1642":
		return "The installer can't install the upgrade patch because the program being upgraded may be missing or the upgrade patch updates a different version of the program. Verify that the program to be upgraded exists on your computer and that you have the correct upgrade patch"
	case "1643":
		return "The patch package isn't permitted by system policy"
	case "1644":
		return "One or more customizations aren't permitted by system policy"
	case "1645":
		return "Windows Installer doesn't permit installation from a Remote Desktop Connection"
	case "1646":
		return "The patch package isn't a removable patch package"
	case "1647":
		return "The patch isn't applied to this product"
	case "1648":
		return "No valid sequence could be found for the set of patches"
	case "1649":
		return "Patch removal was disallowed by policy"
	case "1650":
		return "The XML patch data is invalid"
	case "1651":
		return "Administrative user failed to apply patch for a per-user managed or a per-machine application that'is in advertised state"
	case "1652":
		return "Windows Installer isn't accessible when the computer is in Safe Mode. Exit Safe Mode and try again or try using system restore to return your computer to a previous state. Available beginning with Windows Installer version 4.0"
	case "1653":
		return "Couldn't perform a multiple-package transaction because rollback has been disabled. Multiple-package installations can't run if rollback is disabled. Available beginning with Windows Installer version 4.5"
	case "1654":
		return "The app that you're trying to run isn't supported on this version of Windows. A Windows Installer package, patch, or transform that has not been signed by Microsoft can't be installed on an ARM computer"
	case "3010":
		return "A restart is required to complete the install. This message indicates success. This does not include installs where the ForceReboot action is run"
	default:
		return "Unknown MSI error"
	}
}
