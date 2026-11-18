package agent

import (
	"os"
	"path/filepath"
	"testing"

	openuem_nats "github.com/open-uem/nats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWinGetSourceFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), winGetSourceFilename)
	_, err := readWinGetSource(path)
	assert.Error(t, err, "nothing registered yet")

	cfg := openuem_nats.WinGetSourceConfig{Name: "OpenUEM", URL: "https://console.example.com:1324/winget/agent/token", TrustCA: true}
	require.NoError(t, writeWinGetSource(path, cfg))

	assert.False(t, winGetSourceNeedsRegistration(path, cfg), "nothing changed")

	changed := cfg
	changed.URL = "https://other.example.com:1324/winget/agent/token"
	assert.True(t, winGetSourceNeedsRegistration(path, changed), "a changed URL is registered again")
}

func TestWinGetSourceNeedsRegistration(t *testing.T) {
	path := filepath.Join(t.TempDir(), winGetSourceFilename)
	cfg := openuem_nats.WinGetSourceConfig{Name: "OpenUEM", URL: "https://console.example.com:1323/winget/agent/token", TrustCA: true}
	assert.True(t, winGetSourceNeedsRegistration(path, cfg), "nothing registered yet")

	// Sources registered before the source was trusted are registered again, so winget
	// can install their packages as SYSTEM
	require.NoError(t, os.WriteFile(path, []byte(`{"name":"OpenUEM","url":"https://console.example.com:1323/winget/agent/token","trustca":true}`), 0o600))
	assert.True(t, winGetSourceNeedsRegistration(path, cfg), "registered as untrusted source")

	require.NoError(t, writeWinGetSource(path, cfg))
	assert.False(t, winGetSourceNeedsRegistration(path, cfg), "registered as trusted source")

	// The file is readable by local users, so it doesn't keep the URL with the agent's
	// token, only a hash to notice changes
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "token")
	assert.NotContains(t, string(data), "console.example.com")
}

func TestValidWinGetSourceConfig(t *testing.T) {
	url := "https://console.example.com:1323/winget/agent/token"
	assert.True(t, validWinGetSourceConfig(openuem_nats.WinGetSourceConfig{Name: "OpenUEM", URL: url}))
	// Only the console's own source: a message must not replace winget's sources
	for _, name := range []string{"winget", "msstore", "Other", ""} {
		assert.False(t, validWinGetSourceConfig(openuem_nats.WinGetSourceConfig{Name: name, URL: url}), name)
	}
	assert.False(t, validWinGetSourceConfig(openuem_nats.WinGetSourceConfig{Name: "OpenUEM", URL: "http://console.example.com/winget"}))
}
