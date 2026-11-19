package deploy

import (
	"strings"
)

// wingetLogMarkers select the lines of a winget log that explain a failure: failed calls
// with their HRESULT, exceptions, HTTP responses and how the command ended
var wingetLogMarkers = []string{"[FAIL]", "exception", "response status", "terminating context", "error"}

// FilterWinGetLog returns the last max lines of a winget log that explain why a command
// failed. winget's exit code alone, e.g. "Failed to open the source", doesn't say whether
// the connection, the certificate or the answer failed.
func FilterWinGetLog(log string, max int) []string {
	lines := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(log, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		for _, marker := range wingetLogMarkers {
			if strings.Contains(lower, strings.ToLower(marker)) {
				lines = append(lines, line)
				break
			}
		}
	}
	if len(lines) > max {
		lines = lines[len(lines)-max:]
	}
	return lines
}
