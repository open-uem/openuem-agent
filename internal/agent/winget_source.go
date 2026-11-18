package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
		if err := json.Unmarshal(msg.Data, &cfg); err != nil || !validWinGetSourceConfig(cfg) {
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
	if path != "" && !winGetSourceNeedsRegistration(path, cfg) {
		return
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

// winGetSourceName is the only source the console can register. The source is added as
// trusted for SYSTEM, so a message must not be able to replace winget's own sources.
const winGetSourceName = "OpenUEM"

// validWinGetSourceConfig checks a registration request from the console
func validWinGetSourceConfig(cfg openuem_nats.WinGetSourceConfig) bool {
	return cfg.Name == winGetSourceName && strings.HasPrefix(cfg.URL, "https://")
}

// registeredWinGetSource is the source saved after a registration. The file is readable
// by local users (it's next to the agent), so only a hash of the source is kept: its URL
// contains the agent's token. Trusted is false in files written before sources were
// added as trusted, so those are registered again.
type registeredWinGetSource struct {
	Source  string `json:"source"`
	Trusted bool   `json:"trusted"`
}

func winGetSourceHash(cfg openuem_nats.WinGetSourceConfig) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%t", cfg.Name, cfg.URL, cfg.TrustCA)))
	return hex.EncodeToString(sum[:])
}

// winGetSourceNeedsRegistration reports whether winget has to run: nothing was registered
// yet, the console sent another source, or the source was registered as untrusted.
func winGetSourceNeedsRegistration(path string, cfg openuem_nats.WinGetSourceConfig) bool {
	registered, err := readWinGetSource(path)
	return err != nil || registered.Source != winGetSourceHash(cfg) || !registered.Trusted
}

func readWinGetSource(path string) (registeredWinGetSource, error) {
	registered := registeredWinGetSource{}
	data, err := os.ReadFile(path)
	if err != nil {
		return registered, err
	}
	err = json.Unmarshal(data, &registered)
	return registered, err
}

// writeWinGetSource saves the registered source
func writeWinGetSource(path string, cfg openuem_nats.WinGetSourceConfig) error {
	data, err := json.Marshal(registeredWinGetSource{Source: winGetSourceHash(cfg), Trusted: true})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
