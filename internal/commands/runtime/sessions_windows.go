//go:build windows

package runtime

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// UserSession is a logged-in user's session. Unlike RunAsUser, which borrows the token
// of the first explorer.exe process found, sessions are enumerated using the Terminal
// Services API so every user logged in on the endpoint (e.g. fast user switching or RDS)
// is taken into account.
type UserSession struct {
	ID       uint32
	Username string
	token    windows.Token
	env      []string
}

// GetUserSessions returns the sessions of the users logged in on the endpoint. If a user
// has several sessions only the first one is returned. Sessions must be closed with
// CloseUserSessions. The agent must run as SYSTEM to get the users' tokens.
func GetUserSessions() ([]UserSession, error) {
	var sessionInfo *windows.WTS_SESSION_INFO
	var count uint32

	if err := windows.WTSEnumerateSessions(0, 0, 1, &sessionInfo, &count); err != nil {
		return nil, err
	}
	defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(sessionInfo)))

	sessions := []UserSession{}
	seen := map[string]bool{}
	for _, info := range unsafe.Slice(sessionInfo, count) {
		// Session 0 is reserved for services
		if info.SessionID == 0 || (info.State != windows.WTSActive && info.State != windows.WTSDisconnected) {
			continue
		}

		var token windows.Token
		if err := windows.WTSQueryUserToken(info.SessionID, &token); err != nil {
			// No user is logged in this session
			continue
		}

		username, err := tokenUsername(token)
		if err != nil || seen[username] {
			token.Close()
			continue
		}

		env, err := token.Environ(false)
		if err != nil {
			token.Close()
			continue
		}

		seen[username] = true
		sessions = append(sessions, UserSession{ID: info.SessionID, Username: username, token: token, env: env})
	}

	return sessions, nil
}

// CloseUserSessions releases the tokens held by the sessions
func CloseUserSessions(sessions []UserSession) {
	for _, s := range sessions {
		s.token.Close()
	}
}

// Getenv returns the value of an environment variable for the session's user
func (s UserSession) Getenv(name string) string {
	prefix := strings.ToUpper(name) + "="
	for _, v := range s.env {
		if strings.HasPrefix(strings.ToUpper(v), prefix) {
			return v[len(prefix):]
		}
	}
	return ""
}

// Run executes a hidden command as the session's user, with the user's environment and
// a below normal priority, and returns its combined output. The command is killed if
// it doesn't finish before the timeout.
func (s UserSession) Run(timeout time.Duration, cmdPath string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdPath, args...)
	cmd.Env = s.env
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token:         syscall.Token(s.token),
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.BELOW_NORMAL_PRIORITY_CLASS,
	}

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	return output.Bytes(), err
}

func tokenUsername(token windows.Token) (string, error) {
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}

	account, domain, _, err := user.User.Sid.LookupAccount("")
	if err != nil {
		return "", err
	}

	if domain == "" {
		return account, nil
	}
	return domain + "\\" + account, nil
}
