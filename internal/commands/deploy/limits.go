package deploy

import "strings"

// isLocalDrivePath reports whether a Windows path is an absolute path on a drive, like
// C:\Users. Paths from a user's environment (e.g. LOCALAPPDATA) can point to a network
// share, which the agent, as SYSTEM, would access with the computer's account.
func isLocalDrivePath(p string) bool {
	if len(p) < 3 || p[1] != ':' || (p[2] != '\\' && p[2] != '/') {
		return false
	}
	c := p[0] | 0x20
	return c >= 'a' && c <= 'z' && !strings.Contains(p, "\\\\")
}
