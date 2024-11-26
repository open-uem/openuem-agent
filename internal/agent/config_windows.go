//go:build windows

package agent

import (
	"log"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/sys/windows/registry"
)

const SCHEDULETIME_5MIN = 5

type Config struct {
	NATSHost                 string
	NATSPort                 string
	UUID                     string
	ExecuteTaskEveryXMinutes int
	Enabled                  bool
	Debug                    bool
	DefaultFrequency         int
	VNCProxyPort             string
	SFTPPort                 string
}

func (a *Agent) ReadConfig() {

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\OpenUEM\Agent`, registry.QUERY_VALUE)
	if err != nil {
		log.Println("[ERROR]: agent cannot read the agent hive")
	}
	defer k.Close()

	uuid, _, err := k.GetStringValue("UUID")
	if err == nil {
		a.Config.UUID = uuid
	}

	enabled, _, err := k.GetIntegerValue("Enabled")
	if err == nil {
		a.Config.Enabled = enabled == 1
	}

	scheduled, _, err := k.GetIntegerValue("ExecuteTaskEveryXMinutes")
	if err == nil {
		a.Config.ExecuteTaskEveryXMinutes = int(scheduled)
	}

	serverUrl, _, err := k.GetStringValue("NATSServers")
	if err == nil {
		strippedUrl := strings.Split(serverUrl, ":")
		if len(strippedUrl) == 2 {
			a.Config.NATSHost = strippedUrl[0]
			a.Config.NATSPort = strippedUrl[1]
		}
	}

	debug, _, err := k.GetStringValue("Debug")
	if err == nil {
		val, err := strconv.ParseBool(debug)
		if err != nil {
			a.Config.Debug = false
		} else {
			a.Config.Debug = val
		}
	}

	defaultFrequency, _, err := k.GetIntegerValue("DefaultFrequency")
	if err == nil {
		a.Config.DefaultFrequency = int(defaultFrequency)
	}

	sftpPort, _, err := k.GetStringValue("SFTPPort")
	if err == nil {
		val, err := strconv.Atoi(sftpPort)
		if err != nil || (val < 0) || (val > 65535) {
			a.Config.SFTPPort = ""
		} else {
			a.Config.SFTPPort = sftpPort
		}
	}

	vncPort, _, err := k.GetStringValue("VNCProxyPort")
	if err == nil {
		val, err := strconv.Atoi(vncPort)
		if err != nil || (val < 0) || (val > 65535) {
			a.Config.VNCProxyPort = ""
		} else {
			a.Config.VNCProxyPort = vncPort
		}
	}

	k.Close()
	log.Println("[INFO]: agent has read its settings from the registry")

}

func (c *Config) WriteConfig() {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\OpenUEM\Agent`, registry.SET_VALUE)
	if err != nil {
		log.Println("[ERROR]: agent cannot read the agent hive")
	}
	defer k.Close()

	err = k.SetStringValue("UUID", c.UUID)
	if err != nil {
		log.Println("[ERROR]: could not save the Enabled key")
	}

	enabled := 0
	if c.Enabled {
		enabled = 1
	}

	err = k.SetDWordValue("Enabled", uint32(enabled))
	if err != nil {
		log.Println("[ERROR]: could not save the Enabled key")
	}

	err = k.SetDWordValue("DefaultFrequency", uint32(c.DefaultFrequency))
	if err != nil {
		log.Println("[ERROR]: could not save the Default Frequency key")
	}

	err = k.SetDWordValue("ExecuteTaskEveryXMinutes", uint32(c.ExecuteTaskEveryXMinutes))
	if err != nil {
		log.Println("[ERROR]: could not save the Enabled key")
	}

	log.Println("[INFO]: agent has updated its registry keys")
}

func (a *Agent) SetInitialConfig() {
	id := uuid.New()
	a.Config.UUID = id.String()
	a.Config.Enabled = true
	a.Config.ExecuteTaskEveryXMinutes = 5
	a.Config.WriteConfig()
}
