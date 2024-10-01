package agent

import (
	"log"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/sys/windows/registry"
)

const SCHEDULETIME_5MIN = 5
const SCHEDULETIME_60MIN = 60

type Config struct {
	NATSHost                 string
	NATSPort                 string
	UUID                     string
	ExecuteTaskEveryXMinutes int
	Enabled                  bool
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
	k.Close()

	k, err = registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\OpenUEM`, registry.QUERY_VALUE)
	if err != nil {
		log.Println("[ERROR]: agent cannot read the OpenUEM hive")
		return
	}
	defer k.Close()

	serverUrl, _, err := k.GetStringValue("NATS")
	if err == nil {
		strippedUrl := strings.Split(serverUrl, ":")
		if len(strippedUrl) == 2 {
			a.Config.NATSHost = strippedUrl[0]
			a.Config.NATSPort = strippedUrl[1]
		}
	}

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

	err = k.SetDWordValue("ExecuteTaskEveryXMinutes", uint32(c.ExecuteTaskEveryXMinutes))
	if err != nil {
		log.Println("[ERROR]: could not save the Enabled key")
	}

	log.Println("[INFO]: agent has updated its INI file")
}

func (a *Agent) SetInitialConfig() {
	id := uuid.New()
	a.Config.UUID = id.String()
	a.Config.Enabled = true
	a.Config.ExecuteTaskEveryXMinutes = 5
	a.Config.WriteConfig()
}
