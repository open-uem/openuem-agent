package agent

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"time"

	"github.com/doncicuto/openuem-agent/internal/utils"
)

const JSON_CONFIG = "openuem.json"

type Config struct {
	UUID                 string    `json:"uuid"`
	FirstExecutionDate   time.Time `json:"first_execution_date"`
	LastExecutionDate    time.Time `json:"last_execution_date"`
	LastReportDate       time.Time `json:"last_report_date"`
	ExecuteEveryXMinutes uint8     `json:"execute_every_x_minutes"`
}

func readConfig() Config {
	f, err := os.Open(JSON_CONFIG)
	if os.IsNotExist(err) {
		f, err = os.Create(JSON_CONFIG)
		if err != nil {
			log.Fatal(err)
		}
	}
	defer f.Close()

	byteValue, err := io.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}

	config := Config{}
	_ = json.Unmarshal(byteValue, &config)
	return config
}

func writeConfig(config Config) {
	byteValue, err := json.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}

	_ = os.WriteFile(JSON_CONFIG, byteValue, 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func (c Config) didIReportToday() bool {
	return utils.DateEqual(c.LastReportDate, c.LastExecutionDate)
}
