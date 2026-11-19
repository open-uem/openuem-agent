package deploy

import (
	"errors"
	"strings"
)

// ErrNoApplicableUpdate is returned when a package is already up to date
var ErrNoApplicableUpdate = errors.New("no applicable update found")

// ErrPackageNotInstalled is returned when the package to update isn't installed
var ErrPackageNotInstalled = errors.New("the package is not installed")

// outputTail returns the last lines of a command output, so it can be used as
// additional info about an update without storing the whole log
func outputTail(output string, maxLines int) string {
	lines := []string{}
	for _, line := range cleanWinGetOutput(output) {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}
