package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewestWinGetPath(t *testing.T) {
	root := t.TempDir()
	add := func(dir string, size int) {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
		if size >= 0 {
			require.NoError(t, os.WriteFile(filepath.Join(root, dir, "winget.exe"), make([]byte, size), 0o644))
		}
	}

	_, err := newestWinGetPath(root)
	assert.Error(t, err, "no App Installer")

	add("Microsoft.DesktopAppInstaller_1.9.25200.0_x64__8wekyb3d8bbwe", 100)
	add("Microsoft.DesktopAppInstaller_1.26.509.0_x64__8wekyb3d8bbwe", 100)
	// A newer version staged by an update that isn't complete: its winget.exe is empty
	add("Microsoft.DesktopAppInstaller_1.29.379.0_x64__8wekyb3d8bbwe", 0)
	// Without winget.exe, and for another architecture
	add("Microsoft.DesktopAppInstaller_1.30.1.0_x64__8wekyb3d8bbwe", -1)
	add("Microsoft.DesktopAppInstaller_1.31.1.0_arm64__8wekyb3d8bbwe", 100)
	add("Microsoft.DesktopAppInstaller_2026.917.151.0_neutral_~_8wekyb3d8bbwe", -1)

	path, err := newestWinGetPath(root)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, "Microsoft.DesktopAppInstaller_1.26.509.0_x64__8wekyb3d8bbwe", "winget.exe"), path,
		"the newest version with a usable winget.exe, compared as versions (1.26 > 1.9)")
}
