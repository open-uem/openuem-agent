package deploy

import (
	"testing"

	"github.com/open-uem/nats"
	"github.com/stretchr/testify/assert"
)

func TestParseBrewOutdated(t *testing.T) {
	output := `Warning: some formulae are deprecated
{
  "formulae": [
    {"name": "git", "installed_versions": ["2.46.0"], "current_version": "2.47.0", "pinned": false, "pinned_version": null},
    {"name": "node", "installed_versions": ["20.1.0"], "current_version": "22.0.0", "pinned": true, "pinned_version": "20.1.0"}
  ],
  "casks": [
    {"name": "firefox", "installed_versions": ["131.0"], "current_version": "132.0"},
    {"name": "old-cask", "installed_versions": "1.0", "current_version": "1.1"}
  ]
}`

	updates, err := ParseBrewOutdated([]byte(output))
	assert.NoError(t, err)
	assert.Equal(t, []nats.AvailableUpdate{
		{PackageId: "git", PackageName: "git", PackageBrewType: "formula", InstalledVersion: "2.46.0", AvailableVersion: "2.47.0", Source: "brew"},
		{PackageId: "firefox", PackageName: "firefox", PackageBrewType: "cask", InstalledVersion: "131.0", AvailableVersion: "132.0", Source: "brew"},
		{PackageId: "old-cask", PackageName: "old-cask", PackageBrewType: "cask", InstalledVersion: "1.0", AvailableVersion: "1.1", Source: "brew"},
	}, updates)

	_, err = ParseBrewOutdated([]byte("Error: not json"))
	assert.Error(t, err)
}
