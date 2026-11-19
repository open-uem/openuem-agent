package deploy

import (
	"testing"

	"github.com/open-uem/nats"
	"github.com/stretchr/testify/assert"
)

func TestParseFlatpakUpdates(t *testing.T) {
	remote := "org.mozilla.firefox\tstable\t132.0\n" +
		"org.gimp.GIMP\tstable\t\n"
	installed := "org.mozilla.firefox\tstable\t131.0\tFirefox\n" +
		"org.gimp.GIMP\tstable\t2.10.36\tGNU Image Manipulation Program\n" +
		"org.videolan.VLC\tstable\t3.0.20\tVLC\n"

	assert.Equal(t, []nats.AvailableUpdate{
		{PackageId: "org.mozilla.firefox", PackageName: "Firefox", PackageBranch: "stable", InstalledVersion: "131.0", AvailableVersion: "132.0", Source: "flatpak", Scope: nats.PackageScopeMachine},
		{PackageId: "org.gimp.GIMP", PackageName: "GNU Image Manipulation Program", PackageBranch: "stable", InstalledVersion: "2.10.36", Source: "flatpak", Scope: nats.PackageScopeMachine},
	}, ParseFlatpakUpdates(remote, installed, nats.PackageScopeMachine))

	assert.Empty(t, ParseFlatpakUpdates("", installed, nats.PackageScopeUser))
}
