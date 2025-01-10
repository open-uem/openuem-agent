//go:build windows

package agent

import (
	"log"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
	openuem_utils "github.com/open-uem/utils"
	"gopkg.in/ini.v1"
)

const SCHEDULETIME_5MIN = 5

type Config struct {
	NATSServers              string
	UUID                     string
	ExecuteTaskEveryXMinutes int
	Enabled                  bool
	Debug                    bool
	DefaultFrequency         int
	VNCProxyPort             string
	SFTPPort                 string
	CACert                   string
	AgentCert                string
	AgentKey                 string
	SFTPCert                 string
}

func (a *Agent) ReadConfig() error {
	// Get conf file
	configFile := openuem_utils.GetConfigFile()

	// Open ini file
	cfg, err := ini.Load(configFile)
	if err != nil {
		log.Println("[ERROR]: could not read INI file")
		return err
	}

	key, err := cfg.Section("Agent").GetKey("UUID")
	if err != nil {
		log.Println("[ERROR]: could not get UUID")
		return err
	}
	a.Config.UUID = key.String()

	key, err = cfg.Section("Agent").GetKey("Enabled")
	if err != nil {
		log.Println("[ERROR]: could not get Enabled")
		return err
	}
	a.Config.Enabled, err = key.Bool()
	if err != nil {
		log.Println("[ERROR]: could not parse Enabled")
		return err
	}

	key, err = cfg.Section("Agent").GetKey("ExecuteTaskEveryXMinutes")
	if err != nil {
		log.Println("[ERROR]: could not get ExecuteTaskEveryXMinutes")
		return err
	}
	a.Config.ExecuteTaskEveryXMinutes, err = key.Int()
	if err != nil {
		log.Println("[ERROR]: could not parse ExecuteTaskEveryXMinutes")
		return err
	}

	key, err = cfg.Section("NATS").GetKey("NATSServers")
	if err != nil {
		log.Println("[ERROR]: could not get NATSServers")
		return err
	}
	a.Config.NATSServers = key.String()

	key, err = cfg.Section("Agent").GetKey("Debug")
	if err != nil {
		log.Println("[ERROR]: could not get Debug")
		return err
	}
	a.Config.Debug, err = key.Bool()
	if err != nil {
		log.Println("[ERROR]: could not parse Debug")
		return err
	}

	key, err = cfg.Section("Agent").GetKey("DefaultFrequency")
	if err != nil {
		log.Println("[ERROR]: could not get DefaultFrequency")
		return err
	}
	a.Config.DefaultFrequency, err = key.Int()
	if err != nil {
		log.Println("[ERROR]: could not parse DefaultFrequency")
		return err
	}

	key, err = cfg.Section("Agent").GetKey("SFTPPort")
	if err != nil {
		log.Println("[ERROR]: could not get SFTPPort")
		return err
	}
	a.Config.SFTPPort = key.String()
	val, err := strconv.Atoi(a.Config.SFTPPort)
	if err != nil || (val < 0) || (val > 65535) {
		a.Config.SFTPPort = ""
	}

	key, err = cfg.Section("Agent").GetKey("VNCProxyPort")
	if err != nil {
		log.Println("[ERROR]: could not get VNCProxyPort")
		return err
	}
	a.Config.VNCProxyPort = key.String()
	val, err = strconv.Atoi(a.Config.VNCProxyPort)
	if err != nil || (val < 0) || (val > 65535) {
		a.Config.VNCProxyPort = ""
	}

	// Read required certificates and private key
	cwd, err := Getwd()
	if err != nil {
		log.Fatalf("[FATAL]: could not get current working directory")
	}

	a.Config.AgentCert = filepath.Join(cwd, "certificates", "agent.cer")
	_, err = openuem_utils.ReadPEMCertificate(a.Config.AgentCert)
	if err != nil {
		log.Fatalf("[FATAL]: could not read agent certificate")
	}

	a.Config.AgentKey = filepath.Join(cwd, "certificates", "agent.key")
	_, err = openuem_utils.ReadPEMPrivateKey(a.Config.AgentKey)
	if err != nil {
		log.Fatalf("[FATAL]: could not read agent private key")
	}

	a.Config.CACert = filepath.Join(cwd, "certificates", "ca.cer")
	_, err = openuem_utils.ReadPEMCertificate(a.Config.CACert)
	if err != nil {
		log.Fatalf("[FATAL]: could not read CA certificate")
	}

	a.Config.SFTPCert = filepath.Join(cwd, "certificates", "sftp.cer")
	_, err = openuem_utils.ReadPEMCertificate(a.Config.SFTPCert)
	if err != nil {
		log.Fatalf("[FATAL]: could not read sftp certificate")
	}

	log.Println("[INFO]: agent has read its settings from the INI file")
	return nil
}

func (c *Config) WriteConfig() error {
	// Get conf file
	configFile := openuem_utils.GetConfigFile()

	// Open ini file
	cfg, err := ini.Load(configFile)
	if err != nil {
		return err
	}

	cfg.Section("Agent").Key("UUID").SetValue(c.UUID)
	cfg.Section("Agent").Key("Enabled").SetValue(strconv.FormatBool(c.Enabled))
	cfg.Section("Agent").Key("DefaultFrequency").SetValue(strconv.Itoa(c.DefaultFrequency))
	cfg.Section("Agent").Key("ExecuteTaskEveryXMinutes").SetValue(strconv.Itoa(c.ExecuteTaskEveryXMinutes))
	if err := cfg.SaveTo(configFile); err != nil {
		log.Fatalf("[FATAL]: could not save config file, reason: %v", err)
	}
	log.Printf("[INFO]: config has been saved to %s", configFile)
	return nil
}

func (c *Config) ResetRestartRequiredFlag() error {
	// Get conf file
	configFile := openuem_utils.GetConfigFile()

	// Open ini file
	cfg, err := ini.Load(configFile)
	if err != nil {
		return err
	}

	cfg.Section("Agent").Key("RestartRequired").SetValue("false")
	return cfg.SaveTo(configFile)
}

func (a *Agent) SetInitialConfig() {
	id := uuid.New()
	a.Config.UUID = id.String()
	a.Config.Enabled = true
	a.Config.ExecuteTaskEveryXMinutes = 5
	if err := a.Config.WriteConfig(); err != nil {
		log.Fatalf("[FATAL]: could not write agent config: %v", err)
	}
}
