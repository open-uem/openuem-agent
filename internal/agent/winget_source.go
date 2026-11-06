package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/nats-io/nats.go"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/deploy"
)

const winGetSourceFilename = "winget_source.json"

// winGetSourceLock serializes the registrations, as the console asks again periodically
var winGetSourceLock sync.Mutex

// WinGetSourceSubscribe registers the console's private winget repository when the
// console asks for it. The console asks when the agent is admitted and periodically, so
// the agent only runs winget when the source changed since the last registration.
func (a *Agent) WinGetSourceSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.wingetsource."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		if runtime.GOOS != "windows" {
			respondUpdatePackages(msg, "winget sources are only supported on Windows")
			return
		}

		cfg := openuem_nats.WinGetSourceConfig{}
		if err := json.Unmarshal(msg.Data, &cfg); err != nil || cfg.Name == "" || cfg.URL == "" {
			respondUpdatePackages(msg, "invalid winget source")
			return
		}

		respondUpdatePackages(msg, "")
		go a.registerWinGetSource(cfg)
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent winget source, reason: %v", err)
	}
	return nil
}

func (a *Agent) registerWinGetSource(cfg openuem_nats.WinGetSourceConfig) {
	winGetSourceLock.Lock()
	defer winGetSourceLock.Unlock()

	path := winGetSourcePath()
	if path != "" {
		if registered, err := readWinGetSource(path); err == nil && registered == cfg {
			return
		}
	}

	if err := deploy.RegisterWinGetSource(cfg.Name, cfg.URL, a.Config.CACert, cfg.TrustCA); err != nil {
		log.Printf("[ERROR]: could not register the winget source %s, reason: %v", cfg.Name, err)
		return
	}
	log.Printf("[INFO]: the winget source %s of the console has been registered", cfg.Name)

	if path != "" {
		if err := writeWinGetSource(path, cfg); err != nil {
			log.Printf("[ERROR]: could not save the winget source, reason: %v", err)
		}
	}
}

func winGetSourcePath() string {
	dir, err := agentDataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, winGetSourceFilename)
}

func readWinGetSource(path string) (openuem_nats.WinGetSourceConfig, error) {
	cfg := openuem_nats.WinGetSourceConfig{}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

// writeWinGetSource saves the registered source. The file is only readable by the agent's
// account, as the URL contains the agent's token.
func writeWinGetSource(path string, cfg openuem_nats.WinGetSourceConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
