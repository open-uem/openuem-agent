//go:build windows

package report

import (
	"context"
	"fmt"
	"log"
	"strings"

	openuem_nats "github.com/open-uem/nats"
	"golang.org/x/sys/windows/registry"
)

// TODO - Microsoft Store Apps can't be retrieved from registry
func getApplications(debug bool) (map[string]openuem_nats.Application, error) {
	applications := make(map[string]openuem_nats.Application)

	if err := getApplicationsFromRegistry(applications, registry.LOCAL_MACHINE, APPS, ""); err != nil {
		if debug {
			log.Printf("[DEBUG]: could not get apps information from HKLM, reason: %v", err)
		}
	} else {
		log.Printf("[INFO]: apps information has been retrieved from %s\\%s", "HKLM", APPS)
	}

	if err := getApplicationsFromRegistry(applications, registry.LOCAL_MACHINE, APPS32BITS, ""); err != nil {
		if debug {
			log.Printf("[DEBUG]: could not get apps information from HKLM (32 bits), reason: %v", err)
		}
	} else {
		log.Printf("[INFO]: apps information has been retrieved from %s\\%s", "HKLM", APPS32BITS)
	}

	// Users
	sids, err := GetSIDs()
	if err != nil {
		log.Println("[ERROR]: could not get user SIDs")
		return nil, err
	}

	for _, s := range sids {
		if debug {
			log.Printf("[DEBUG]: apps information has been requested for %s", "HKCU\\APPS")
		}
		if err := getApplicationsFromRegistry(applications, registry.USERS, APPS, s.SID); err != nil {
			if debug {
				log.Printf("[DEBUG]: could not get apps information from HKEY_USERS for sid %s, reason: %v", s, err)
			}
			continue
		}
		log.Printf("[INFO]: apps information retrieved from HKEY_USERS for sid %s\n", s)

		if debug {
			log.Printf("[DEBUG]: apps information has been requested for %s", "HKCU\\APPS32BITS")
		}
		if err := getApplicationsFromRegistry(applications, registry.USERS, APPS32BITS, s.SID); err != nil {
			if debug {
				log.Printf("[DEBUG]: could not get apps information from HKEY_USERS (32bits) for sid %s, reason: %v", s, err)
			}
			continue
		}
		log.Printf("[INFO]: apps information retrieved from HKEY_USERS (32 bits) for sid %s\n", s)
	}

	return applications, nil
}

func getApplicationsFromRegistry(applications map[string]openuem_nats.Application, hive registry.Key, key, sid string) error {

	if hive == registry.USERS {
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
			applications[displayName] = openuem_nats.Application{Version: displayVersion, InstallDate: installDate, Publisher: publisher}
		}
	}
	return nil
}

func GetSID(username string) (string, error) {
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

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qSID, &response, namespace)
	if err != nil {
		log.Printf("[ERROR]: could not generate SQL for WMI Win32_UserAccount: %v", err)
		return "", err
	}

	if len(response) != 1 {
		log.Printf("[ERROR]: expected one result got %d: %v", len(response), err)
		return "", err
	}

	return response[0].SID, nil
}

func GetSIDs() ([]struct{ SID string }, error) {
	var response []struct{ SID string }

	// This query would not be acceptable in general as it could lead to sql injection, but we're using a where condition using a
	// index value retrieved by WMI it's not user generated input
	namespace := `root\cimv2`

	qSID := "SELECT SID FROM Win32_UserAccount"

	ctx := context.Background()
	err := WMIQueryWithContext(ctx, qSID, &response, namespace)
	if err != nil {
		log.Printf("[ERROR]: could not generate SQL for WMI Win32_UserAccount: %v", err)
		return nil, err
	}

	return response, nil
}
