package agent

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/doncicuto/openuem-agent/internal/utils"
	"gopkg.in/ini.v1"
)

const JSON_CONFIG = "openuem.ini"

type Config struct {
	ServerUrl            string    `ini:"ServerUrl"`
	UUID                 string    `ini:"UUID" json:"uuid"`
	FirstExecutionDate   time.Time `ini:"FirstExecutionDate" json:"first_execution_date"`
	LastExecutionDate    time.Time `ini:"LastExecutionDate" json:"last_execution_date"`
	LastReportDate       time.Time `ini:"LastReportDate" json:"last_report_date"`
	ExecuteEveryXMinutes int       `ini:"ExecuteEveryXMinutes" json:"execute_every_x_minutes"`
}

func (c *Config) ReadConfig() {
	cwd, err := utils.Getwd()
	if err != nil {
		log.Fatalf("could not get cwd: %v", err)
	}

	path := filepath.Join(cwd, "config", JSON_CONFIG)

	// Check if file exists and create if not
	if _, err := os.Stat(path); os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			log.Printf("[ERROR]: could not create file in path: %s - %v", path, err)
		}
		defer f.Close()
	}

	// Try to read INI file
	cfg, err := ini.Load(path)
	if err != nil {
		log.Fatalf("could not read ini file: %v", err)
	}

	// Map content to structure
	err = cfg.Section("Config").MapTo(&c)
	if err != nil {
		log.Fatalf("could not parse ini file: %v", err)
	}
	log.Println("[INFO]: agent has read its INI file")

	err = os.Chdir("../")
	if err != nil {
		log.Fatalf("could not change to parent folder: %v", err)
	}
}

func (c *Config) WriteConfig() {
	cwd, err := utils.Getwd()
	if err != nil {
		log.Fatalf("could not get cwd: %v", err)
	}

	path := filepath.Join(cwd, "config", JSON_CONFIG)

	cfg, err := ini.Load(path)
	if err != nil {
		log.Fatalf("could not read ini file: %v", err)
	}

	err = cfg.Section("Config").ReflectFrom(&c)
	if err != nil {
		log.Fatalf("could not reflect ini from config: %v", err)
	}

	err = cfg.SaveTo(path)
	if err != nil {
		log.Fatalf("could not save ini: %v", err)
	}
	log.Println("[INFO]: agent has updated its INI file")
}

func (c Config) DidIReportToday() bool {
	return utils.DateEqual(c.LastReportDate, c.LastExecutionDate)
}
