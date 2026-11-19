package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWinGetSourceAddArgs(t *testing.T) {
	args := wingetSourceAddArgs("OpenUEM", "https://console.example.com:1323/winget/agent/token")
	assert.Equal(t, []string{"source", "add", "--name", "OpenUEM", "--arg", "https://console.example.com:1323/winget/agent/token",
		"--type", "Microsoft.Rest", "--trust-level", "trusted", "--accept-source-agreements", "--disable-interactivity", "--verbose-logs"}, args)
}
