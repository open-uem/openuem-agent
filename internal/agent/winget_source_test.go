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

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"OpenUEM","url":"https://console.example.com:1323/winget/agent/token","trustca":true,"trusted":true}`, string(data))
}
