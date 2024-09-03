package agent

import (
	"fmt"
	"log"
	"strings"

	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows/registry"
)

const (
	APPS32BITS = `SOFTWARE\Wow6432Node\Microsoft\Windows\CurrentVersion\Uninstall`
	APPS       = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
)

type Application struct {
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
	InstallDate string `json:"install_date,omitempty"`
	Publisher   string `json:"publisher,omitempty"`
}

func (a *Agent) getApplicationsInfo() {
	a.Edges.Applications = []Application{}
	myApps := getApplications()
	for k, v := range myApps {
		app := Application{}
		app.Name = strings.TrimSpace(k)
		app.Version = strings.TrimSpace(v.Version)
		app.InstallDate = strings.TrimSpace(v.InstallDate)
		app.Publisher = strings.TrimSpace(v.Publisher)
		a.Edges.Applications = append(a.Edges.Applications, app)
	}

}

func (a *Agent) logApplications() {
	fmt.Printf("\n** 📱 Software ******************************************************************************************************\n")
	if len(a.Edges.Applications) > 0 {
		for i, v := range a.Edges.Applications {
			fmt.Printf("%-40s |  %s \n", "Application", v.Name)
			fmt.Printf("%-40s |  %s \n", "Version", v.Version)
			fmt.Printf("%-40s |  %s \n", "Publisher", v.Publisher)
			fmt.Printf("%-40s |  %s \n", "Installation date", v.InstallDate)
			if len(a.Edges.Applications) > 1 && i+1 != len(a.Edges.Applications) {
				fmt.Printf("---------------------------------------------------------------------------------------------------------------------\n")
			}
		}
	} else {
		fmt.Printf("%-40s\n", "No applications found")
	}
}

// TODO - Microsoft Store Apps can't be retrieved from registry
func getApplications() map[string]Application {
	applications := make(map[string]Application)

	if err := getApplicationsFromRegistry(applications, registry.LOCAL_MACHINE, APPS); err != nil {
		log.Printf("[WARN]: could not get apps information from %s\\%s: %v", "HKLM", APPS, err)
	} else {
		log.Printf("[INFO]: apps information has been retrieved from %s\\%s", "HKLM", APPS)
	}

	if err := getApplicationsFromRegistry(applications, registry.LOCAL_MACHINE, APPS32BITS); err != nil {
		log.Printf("[WARN]: could not get apps information from %s\\%s: %v", "HKLM", APPS32BITS, err)
	} else {
		log.Printf("[INFO]: apps information has been retrieved from %s\\%s", "HKLM", APPS)
	}

	if err := getApplicationsFromRegistry(applications, registry.USERS, APPS); err != nil {
		log.Printf("[WARN]: could not get apps information from %s\\%s: %v", "HKCU", APPS, err)
	} else {
		log.Printf("[INFO]: apps information has been retrieved from %s\\%s", "HKCU", APPS)
	}

	if err := getApplicationsFromRegistry(applications, registry.USERS, APPS32BITS); err != nil {
		log.Printf("[WARN]: could not get apps information from %s\\%s: %v", "HKCU", APPS32BITS, err)
	} else {
		log.Printf("[INFO]: apps information has been retrieved from %s\\%s", "HKCU", APPS)
	}
	return applications
}

func getApplicationsFromRegistry(applications map[string]Application, hive registry.Key, key string) error {

	if hive == registry.USERS {
		loggedOnUser, err := getLoggedOnUsername()
		if err != nil {
			return fmt.Errorf("could not get logged on username")
		}

		sid, err := getSID(loggedOnUser)
		if err != nil {
			return fmt.Errorf("could not get SID for logged on user")
		}

		key = fmt.Sprintf("%s\\%s", sid, key)
	}

	k, err := registry.OpenKey(hive, key, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return err
	}
	defer k.Close()

	names, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		sk, err := registry.OpenKey(hive, fmt.Sprintf("%s\\%s", key, name), registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		defer sk.Close()
		displayName, _, err := sk.GetStringValue("DisplayName")
		_, ok := applications[displayName]
		if err == nil && !ok {
			displayVersion, _, err := sk.GetStringValue("DisplayVersion")
			if err != nil {
				continue
			}
			installDate, _, _ := sk.GetStringValue("InstallDate")
			publisher, _, _ := sk.GetStringValue("Publisher")
			applications[displayName] = Application{Version: displayVersion, InstallDate: installDate, Publisher: publisher}
		}
	}
	return nil
}

func getSID(username string) (string, error) {
	var response []struct{ SID string }

	// This query would not be acceptable in general as it could lead to sql injection, but we're using a where condition using a
	// index value retrieved by WMI it's not user generated input
	namespace := `root\cimv2`

	user := strings.Split(username, "\\")

	if len(user) != 2 {
		log.Println("[ERROR]: could not parse username for WMI Win32_UserAccount query")
		return "", fmt.Errorf("could not parse username, expect a domain and a name")
	}

	qSID := fmt.Sprintf("SELECT SID FROM Win32_UserAccount WHERE Domain = '%s' and Name = '%s'", user[0], user[1])
	err := wmi.QueryNamespace(qSID, &response, namespace)
	if err != nil {
		log.Printf("[ERROR]: could not generate SQL for WMI Win32_UserAccount: %v", err)
		return "", err
	}

	err = wmi.QueryNamespace(qSID, &response, namespace)
	if err != nil {
		log.Printf("[ERROR]: could not get user account info from WMI Win32_UserAccount: %v", err)
		return "", err
	}

	if len(response) != 1 {
		log.Printf("[ERROR]: expected one result got %d: %v", len(response), err)
		return "", err
	}

	return response[0].SID, nil
}
