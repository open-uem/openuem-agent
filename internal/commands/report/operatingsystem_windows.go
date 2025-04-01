//go:build windows

package report

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	openuem_nats "github.com/open-uem/nats"
	"golang.org/x/sys/windows/registry"
)

type windowsVersion struct {
	name    string
	version string
}

const MAX_DISPLAYNAME_LENGTH = 256

func (r *Report) getOSInfo(debug bool) error {
	if debug {
		log.Println("[DEBUG]: os info (operating system info) has been requested")
	}
	r.OperatingSystem = openuem_nats.OperatingSystem{}
	if err := r.getOperatingSystemInfo(debug); err != nil {
		log.Printf("[ERROR]: could not get operating system info from WMI Win32_OperatingSystem: %v", err)
		return err
	} else {
		log.Printf("[INFO]: operating system information has been retrieved using WMI Win32_OperatingSystem")
	}

	if debug {
		log.Println("[DEBUG]: os edition has been requested")
	}
	if err := r.getEdition(); err != nil {
		log.Printf("[ERROR]: could not get current version from Windows Registry: %v", err)
		return err
	} else {
		log.Printf("[INFO]: current version has been retrieved from Windows Registry")
	}

	if debug {
		log.Println("[DEBUG]: arch info has been requested")
	}
	if err := r.getArch(); err != nil {
		log.Printf("[ERROR]: could not get windows arch from Windows Registry: %v", err)
		return err
	} else {
		log.Printf("[INFO]: windows arch has been retrieved from Windows Registry")
	}

	if debug {
		log.Println("[DEBUG]: username info has been requested")
	}
	if err := r.getUsername(); err != nil {
		log.Printf("[ERROR]: could not get windows username from WMI Win32_Computer: %v", err)
		return err
	} else {
		log.Printf("[INFO]: windows username has been retrieved from WMI Win32_Computer")
	}
	return nil
}

func (r *Report) getOperatingSystemInfo(debug bool) error {
	var osDst []struct {
		Version        string
		Caption        string
		InstallDate    time.Time
		LastBootUpTime time.Time
	}

	if debug {
		log.Println("[DEBUG]: operating system info has been requested")
	}

	namespace := `root\cimv2`
	qOS := "SELECT Version, Caption, InstallDate, LastBootUpTime FROM Win32_OperatingSystem"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qOS, &osDst, namespace)
	if err != nil {
		return err
	}

	if len(osDst) != 1 {
		return fmt.Errorf("got wrong operation system configuration result set")
	}

	v := &osDst[0]
	r.OperatingSystem.Version = "Undetected"
	if v.Version != "" {
		nt := getWindowsVersion(v.Version)
		if nt != nil {
			r.OperatingSystem.Version = fmt.Sprintf("%s %s", nt.name, nt.version)
		}
	}

	r.OperatingSystem.Description = v.Caption
	r.OperatingSystem.InstallDate = v.InstallDate.Local()
	r.OperatingSystem.LastBootUpTime = v.LastBootUpTime.Local()

	return nil
}

func (r *Report) getEdition() error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	s, _, err := k.GetStringValue("EditionID")
	if err != nil {
		return err
	}
	r.OperatingSystem.Edition = s
	return nil
}

func (r *Report) getArch() error {
	r.OperatingSystem.Arch = "Undetected"

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	s, _, err := k.GetStringValue("PROCESSOR_ARCHITECTURE")

	if err != nil {
		return err
	}
	switch s {
	case "AMD64":
		r.OperatingSystem.Arch = "64 bits"
	case "x86":
		r.OperatingSystem.Arch = "32 bits"
	}
	return nil
}

func (r *Report) getUsername() error {
	username, err := GetLoggedOnUsername()
	if err != nil {
		return err
	}
	r.OperatingSystem.Username = strings.TrimSpace(username)
	return nil
}

func GetLoggedOnUsername() (string, error) {
	var computerDst []struct{ Username string }

	namespace := `root\cimv2`
	qComputer := "SELECT Username FROM Win32_ComputerSystem"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qComputer, &computerDst, namespace)
	if err != nil {
		return "", err
	}

	if len(computerDst) != 1 {
		return "", fmt.Errorf("got wrong computer system result set")
	}

	v := &computerDst[0]
	return strings.TrimSpace(v.Username), nil
}

// func (r *Report) isWindowsServer() bool {
// 	return strings.HasPrefix(r.OperatingSystem.Version, "Windows Server")
// }

func getWindowsVersion(version string) *windowsVersion {
	var windowsVersions = map[string]windowsVersion{}

	// Windows 11
	windowsVersions["10.0.26100"] = windowsVersion{name: "Windows 11", version: "24H2"}
	windowsVersions["10.0.22631"] = windowsVersion{name: "Windows 11", version: "23H2"}
	windowsVersions["10.0.22621"] = windowsVersion{name: "Windows 11", version: "22H2"}
	windowsVersions["10.0.22000"] = windowsVersion{name: "Windows 11", version: "21H2"}

	// Windows 10
	windowsVersions["10.0.19045"] = windowsVersion{name: "Windows 10", version: "22H2"}
	windowsVersions["10.0.19044"] = windowsVersion{name: "Windows 10", version: "21H2"}
	windowsVersions["10.0.19043"] = windowsVersion{name: "Windows 10", version: "21H1"}
	windowsVersions["10.0.19042"] = windowsVersion{name: "Windows 10", version: "20H2"}
	windowsVersions["10.0.19041"] = windowsVersion{name: "Windows 10", version: "2004"}
	windowsVersions["10.0.18363"] = windowsVersion{name: "Windows 10", version: "1909"}
	windowsVersions["10.0.18362"] = windowsVersion{name: "Windows 10", version: "1903"}
	windowsVersions["10.0.17763"] = windowsVersion{name: "Windows 10", version: "1809"}
	windowsVersions["10.0.17134"] = windowsVersion{name: "Windows 10", version: "1803"}
	windowsVersions["10.0.16299"] = windowsVersion{name: "Windows 10", version: "1709"}
	windowsVersions["10.0.15063"] = windowsVersion{name: "Windows 10", version: "1703"}
	windowsVersions["10.0.14393"] = windowsVersion{name: "Windows 10", version: "1607"}
	windowsVersions["10.0.10586"] = windowsVersion{name: "Windows 10", version: "1511"}
	windowsVersions["10.0.10240"] = windowsVersion{name: "Windows 10", version: "1507"}

	// Windows 8
	windowsVersions["6.3.9600"] = windowsVersion{name: "Windows 8.1", version: ""}

	// Windows 8.1
	windowsVersions["6.2.9200"] = windowsVersion{name: "Windows 8", version: ""}

	// Windows 7
	windowsVersions["6.1.7601"] = windowsVersion{name: "Windows 7", version: ""}

	// Windows Vista
	windowsVersions["6.0.6002"] = windowsVersion{name: "Windows Vista", version: ""}

	// Windows XP
	windowsVersions["5.1.3790"] = windowsVersion{name: "Windows XP", version: ""}
	windowsVersions["5.1.2710"] = windowsVersion{name: "Windows XP", version: ""}
	windowsVersions["5.1.2700"] = windowsVersion{name: "Windows XP", version: ""}
	windowsVersions["5.1.2600"] = windowsVersion{name: "Windows XP", version: ""}

	// Windows Server 2025

	// Windows Server 2022
	windowsVersions["10.0.25398"] = windowsVersion{name: "Windows Server 2022", version: "23H2"}
	windowsVersions["10.0.20348"] = windowsVersion{name: "Windows Server 2022", version: "21H2"}

	// Windows Server 2019
	windowsVersions["10.0.19042"] = windowsVersion{name: "Windows Server 2019", version: "20H2"}
	windowsVersions["10.0.19041"] = windowsVersion{name: "Windows Server 2019", version: "2004"}
	windowsVersions["10.0.18363"] = windowsVersion{name: "Windows Server 2019", version: "1909"}
	windowsVersions["10.0.18362"] = windowsVersion{name: "Windows Server 2019", version: "1903"}
	windowsVersions["10.0.17763"] = windowsVersion{name: "Windows Server 2019", version: "1809"}

	// Windows Server 2016
	windowsVersions["10.0.17134"] = windowsVersion{name: "Windows Server 2016", version: "1803"}
	windowsVersions["10.0.16299"] = windowsVersion{name: "Windows Server 2016", version: "1709"}
	windowsVersions["10.0.14393"] = windowsVersion{name: "Windows Server 2016", version: "1607"}

	// Windows Server 2012 R2
	windowsVersions["6.3.9600"] = windowsVersion{name: "Windows Server 2012 R2", version: ""}

	// Windows Server 2012
	windowsVersions["6.2.9200"] = windowsVersion{name: "Windows Server 2012", version: ""}

	// Windows Server 2008 R2
	windowsVersions["6.1.7601"] = windowsVersion{name: "Windows Server 2008 R2", version: ""}

	// Windows Server 2008
	windowsVersions["6.0.6003"] = windowsVersion{name: "Windows Server 2008", version: ""}

	// Windows Server 2003
	windowsVersions["5.2.3790"] = windowsVersion{name: "Windows Server 2003", version: ""}

	// Windows 2000
	windowsVersions["5.0.2195"] = windowsVersion{name: "Windows 2000", version: ""}

	val, ok := windowsVersions[version]
	if !ok {
		return nil
	} else {
		return &val
	}
}
