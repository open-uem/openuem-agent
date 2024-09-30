package agent

import (
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"gopkg.in/ini.v1"
)

const JSON_CONFIG = "openuem.ini"
const SCHEDULETIME_5MIN = 5
const SCHEDULETIME_60MIN = 60

type Config struct {
	ServerUrl                string `ini:"ServerUrl"`
	UUID                     string `ini:"UUID" json:"uuid"`
	ExecuteTaskEveryXMinutes int    `ini:"ExecuteEveryXMinutes" json:"execute_every_x_minutes"`
	Enabled                  bool   `ini:"enable" json:"enable"`
}

func (a *Agent) ReadConfig() {
	var err error
	cwd, err := Getwd()
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
	err = cfg.Section("Config").MapTo(&a.Config)
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
	cwd, err := Getwd()
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

func (a *Agent) SetInitialConfig() {
	id := uuid.New()
	a.Config.UUID = id.String()
	a.Config.Enabled = true
	a.Config.WriteConfig()
}
