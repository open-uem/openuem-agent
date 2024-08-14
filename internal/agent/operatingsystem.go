package agent

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/doncicuto/openuem-agent/internal/log"
	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type OperatingSystem struct {
	Version        string    `json:"version,omitempty"`
	Description    string    `json:"description,omitempty"`
	InstallDate    time.Time `json:"install_date,omitempty"`
	Edition        string    `json:"edition,omitempty"`
	Arch           string    `json:"arch,omitempty"`
	Username       string    `json:"username,omitempty"`
	LastBootUpTime time.Time `json:"last_bootup_time,omitempty"`
}

type windowsVersion struct {
	name    string
	version string
}

const MAX_DISPLAYNAME_LENGTH = 256

func (a *Agent) getOSInfo() {
	a.Edges.OperatingSystem = OperatingSystem{}
	if err := a.Edges.OperatingSystem.getOperatingSystemInfo(); err != nil {
		log.Logger.Printf("[ERROR]: could not get operating system info from WMI Win32_OperatingSystem: %v", err)
	} else {
		log.Logger.Printf("[INFO]: operating system information has been retrieved using WMI Win32_OperatingSystem")
	}
	if err := a.Edges.OperatingSystem.getEdition(); err != nil {
		log.Logger.Printf("[ERROR]: could not get current version from Windows Registry: %v", err)
	} else {
		log.Logger.Printf("[INFO]: current version has been retrieved from Windows Registry")
	}
	if err := a.Edges.OperatingSystem.getArch(); err != nil {
		log.Logger.Printf("[ERROR]: could not get windows arch from Windows Registry: %v", err)
	} else {
		log.Logger.Printf("[INFO]: windows arch has been retrieved from Windows Registry")
	}
	if err := a.Edges.OperatingSystem.getUsername(); err != nil {
		log.Logger.Printf("[ERROR]: could not get windows username from Windows Registry: %v", err)
	} else {
		log.Logger.Printf("[INFO]: windows username has been retrieved from Windows Registry")
	}
}

func (a *Agent) logOS() {
	fmt.Printf("\n** 📔 Operating System **********************************************************************************************\n")
	fmt.Printf("%-40s |  %s \n", "Windows Version", a.Edges.OperatingSystem.Version)
	fmt.Printf("%-40s |  %s \n", "Windows Description", a.Edges.OperatingSystem.Description)
	fmt.Printf("%-40s |  %s \n", "Install Date", a.Edges.OperatingSystem.InstallDate)
	fmt.Printf("%-40s |  %s \n", "Windows Edition", a.Edges.OperatingSystem.Edition)
	fmt.Printf("%-40s |  %s \n", "Windows Architecture", a.Edges.OperatingSystem.Arch)
	fmt.Printf("%-40s |  %s \n", "Last Boot Up Time", a.Edges.OperatingSystem.LastBootUpTime)
	fmt.Printf("%-40s |  %s \n", "User Name", a.Edges.OperatingSystem.Username)
}

func (myOs *OperatingSystem) getOperatingSystemInfo() error {
	var osDst []struct {
		Version        string
		Caption        string
		InstallDate    time.Time
		LastBootUpTime time.Time
	}

	namespace := `root\cimv2`
	qOS := "SELECT Version, Caption, InstallDate, LastBootUpTime FROM Win32_OperatingSystem"
	err := wmi.QueryNamespace(qOS, &osDst, namespace)
	if err != nil {
		return err
	}

	if len(osDst) != 1 {
		return fmt.Errorf("got wrong operation system configuration result set")
	}

	v := &osDst[0]
	myOs.Version = "Undetected"
	if v.Version != "" {
		nt, err := getWindowsVersion(v.Version)
		if err != nil {
			return err
		}
		myOs.Version = fmt.Sprintf("%s %s", nt.name, nt.version)
	}

	myOs.Description = v.Caption
	myOs.InstallDate = v.InstallDate.Local()
	myOs.LastBootUpTime = v.LastBootUpTime.Local()

	return nil
}

func (myOs *OperatingSystem) getEdition() error {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	s, _, err := k.GetStringValue("EditionID")
	if err != nil {
		return err
	}
	myOs.Edition = s
	return nil
}

func (myOs *OperatingSystem) getArch() error {
	myOs.Arch = "Undetected"

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
		myOs.Arch = "64 bits"
	case "x86":
		myOs.Arch = "32 bits"
	}
	return nil
}

func (myOs *OperatingSystem) getUsername() error {
	var n uint32 = MAX_DISPLAYNAME_LENGTH
	myOs.Username = "Undetected"

	b := make([]uint16, n)
	err := windows.GetUserNameEx(windows.NameDisplay, &b[0], &n)
	if err != nil {
		return err
	}
	myOs.Username = windows.UTF16ToString(b)
	return nil
}

func (myOs *OperatingSystem) isWindowsServer() bool {
	return strings.HasPrefix(myOs.Version, "Windows Server")
}

func getWindowsVersion(version string) (*windowsVersion, error) {
	var windowsVersions = map[string]windowsVersion{}

	// Windows 11
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
		err := errors.New("windows version not found")
		return nil, err
	} else {
		return &val, nil
	}
}
