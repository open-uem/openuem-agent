package deploy

import (
	"strings"

	"github.com/open-uem/nats"
)

// FlatpakUpdateColumns are the columns requested to `flatpak remote-ls --updates`
var FlatpakUpdateColumns = []string{"--app", "--columns=application,branch,version"}

// FlatpakInstalledColumns are the columns requested to `flatpak list`
var FlatpakInstalledColumns = []string{"--app", "--columns=application,branch,version,name"}

// ParseFlatpakUpdates merges the output of `flatpak remote-ls --updates` with the output
// of `flatpak list` to know the name and installed version of each app with an update.
// When stdout is not a terminal, flatpak prints tab separated columns without a header.
func ParseFlatpakUpdates(remoteUpdates, installed, scope string) []nats.AvailableUpdate {
	type installedApp struct{ name, version string }
	apps := map[string]installedApp{}
	for _, fields := range splitTabbedLines(installed) {
		if len(fields) >= 4 {
			apps[fields[0]+"//"+fields[1]] = installedApp{name: fields[3], version: fields[2]}
		}
	}

	updates := []nats.AvailableUpdate{}
	for _, fields := range splitTabbedLines(remoteUpdates) {
		if len(fields) < 2 || !IsValidPackageID(fields[0]) {
			continue
		}

		u := nats.AvailableUpdate{
			PackageId:     fields[0],
			PackageName:   fields[0],
			PackageBranch: fields[1],
			Source:        "flatpak",
			Scope:         scope,
		}
		if len(fields) >= 3 {
			u.AvailableVersion = fields[2]
		}
		if app, ok := apps[fields[0]+"//"+fields[1]]; ok {
			u.PackageName = app.name
			u.InstalledVersion = app.version
		}
		updates = append(updates, u)
	}

	return updates
}

func splitTabbedLines(output string) [][]string {
	rows := [][]string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		rows = append(rows, fields)
	}
	return rows
}
