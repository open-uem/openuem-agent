package agent

import (
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

	registered, err := readWinGetSource(path)
	require.NoError(t, err)
	assert.Equal(t, cfg, registered)

	changed := cfg
	changed.URL = "https://other.example.com:1324/winget/agent/token"
	assert.NotEqual(t, changed, registered, "a changed URL is registered again")
}
