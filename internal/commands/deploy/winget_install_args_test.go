package deploy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWinGetInstallArgs(t *testing.T) {
	assert.Equal(t, []string{"install", "--id", "7zip.7zip", "--exact", "--scope", "machine", "--silent",
		"--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}, wingetInstallArgs("7zip.7zip", ""))
	assert.Equal(t, []string{"install", "--id", "7zip.7zip", "--exact", "--version", "24.08", "--scope", "machine", "--silent",
		"--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}, wingetInstallArgs("7zip.7zip", "24.08"))
}

func TestWinGetUninstallArgs(t *testing.T) {
	assert.Equal(t, []string{"uninstall", "--id", "7zip.7zip", "--exact", "--all-versions", "--silent",
		"--accept-source-agreements", "--disable-interactivity"}, wingetUninstallArgs("7zip.7zip"))
}

func TestIsValidPackageVersion(t *testing.T) {
	for _, v := range []string{"", "24.08", "1.2.3-beta+4", "2024_01", "v1.0"} {
		assert.True(t, IsValidPackageVersion(v), v)
	}
	for _, v := range []string{"-1", "1.0; calc", "1 0", "$(x)", "1.0'"} {
		assert.False(t, IsValidPackageVersion(v), v)
	}
}
