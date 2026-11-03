//go:build windows

package deploy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	"github.com/open-uem/wingetcfg/wingetcfg"
	"golang.org/x/sys/windows"
)

const (
	// Listing is done as SYSTEM and in every user session, one after another
	wingetListTimeout    = 3 * time.Minute
	wingetUpgradeTimeout = 60 * time.Minute

	wingetNoApplicableUpdate = "0x8A15002B"
	wingetNoPackagesFound    = "0x8A150014"
)

var wingetListArgs = []string{"upgrade", "--accept-source-agreements", "--disable-interactivity"}

func wingetUpgradeArgs(packageID, scope string) []string {
	args := []string{"upgrade", "--id", packageID, "--exact", "--silent", "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}
	// A user session can only update packages installed for that user, so the
	// scope is only forced for machine-wide upgrades run as SYSTEM
	if scope == nats.PackageScopeMachine {
		args = append(args, "--scope", "machine")
	}
	return args
}

// ListAvailableUpdates returns the packages that winget can upgrade. Packages found
// as SYSTEM are machine-scoped; packages found only in a user session are user-scoped.
func ListAvailableUpdates() ([]nats.AvailableUpdate, error) {
	wgPath, err := locateWinGet()
	if err != nil {
		return nil, fmt.Errorf("could not locate winget.exe, reason: %v", err)
	}

	// winget returns a non-zero exit code if, for instance, a source can't be
	// reached, but it may still print the packages it could check
	output, err := runWinGet(wingetListTimeout, wgPath, wingetListArgs...)
	updates := ParseWinGetUpgradeOutput(output)
	if err = ignoreNoUpdatesError(err); err != nil {
		if len(updates) == 0 {
			return nil, fmt.Errorf("winget could not list the available updates, reason: %v", err)
		}
		// Without the complete list of machine-wide packages, packages found in a user
		// session can't be told apart from machine-wide ones, so users aren't checked
		log.Printf("[WARN]: winget could only list some updates, user-scoped packages won't be checked, reason: %v", err)
		for i := range updates {
			updates[i].Scope = nats.PackageScopeMachine
		}
		return updates, nil
	}

	machineIDs := map[string]bool{}
	for i := range updates {
		updates[i].Scope = nats.PackageScopeMachine
		machineIDs[updates[i].PackageId] = true
	}

	sessions, err := runtime.GetUserSessions()
	if err != nil {
		log.Printf("[ERROR]: could not get the logged-in users, user-scoped packages won't be checked, reason: %v", err)
		return updates, nil
	}
	defer runtime.CloseUserSessions(sessions)

	userUpdates := []nats.AvailableUpdate{}
	userUpdateIndex := map[string]int{}
	for _, s := range sessions {
		wgUserPath := userWinGetPath(s)
		if wgUserPath == "" {
			log.Printf("[WARN]: winget.exe was not found for user %s", s.Username)
			continue
		}

		output, err := s.Run(wingetListTimeout, wgUserPath, wingetListArgs...)
		if err = ignoreNoUpdatesError(wingetError(err)); err != nil {
			log.Printf("[ERROR]: could not list updates for user %s, reason: %v", s.Username, err)
			if !isExitError(err) {
				continue
			}
		}

		for _, u := range ParseWinGetUpgradeOutput(string(output)) {
			if machineIDs[u.PackageId] {
				continue
			}
			if i, ok := userUpdateIndex[u.PackageId]; ok {
				userUpdates[i].Users = append(userUpdates[i].Users, s.Username)
				continue
			}
			u.Scope = nats.PackageScopeUser
			u.Users = []string{s.Username}
			userUpdateIndex[u.PackageId] = len(userUpdates)
			userUpdates = append(userUpdates, u)
		}
	}

	return append(updates, userUpdates...), nil
}

// updateUserPackage upgrades a user-scoped package in the session of every logged-in user
// that has it installed. Users that don't have the package are skipped.
func updateUserPackage(packageID string) (string, string, error) {
	sessions, err := runtime.GetUserSessions()
	if err != nil {
		return "", "", fmt.Errorf("could not get the logged-in users, reason: %v", err)
	}
	defer runtime.CloseUserSessions(sessions)

	if len(sessions) == 0 {
		return "", "", errNoLoggedInUsers
	}

	updated := []string{}
	failures := []string{}
	outputs := []string{}
	installed := false
	wingetFound := false
	for _, s := range sessions {
		wgUserPath := userWinGetPath(s)
		if wgUserPath == "" {
			continue
		}
		wingetFound = true

		log.Printf("[INFO]: winget.exe is upgrading the app %s for user %s\n", packageID, s.Username)
		output, err := s.Run(wingetUpgradeTimeout, wgUserPath, wingetUpgradeArgs(packageID, nats.PackageScopeUser)...)
		err = wingetError(err)
		switch {
		case err == nil:
			installed = true
			updated = append(updated, s.Username)
		case errors.Is(err, ErrNoApplicableUpdate):
			installed = true
		case errors.Is(err, errWinGetNoPackagesFound):
			// The user doesn't have this package
		default:
			installed = true
			log.Printf("[ERROR]: winget.exe could not upgrade %s for user %s, reason: %v", packageID, s.Username, err)
			failures = append(failures, fmt.Sprintf("%s: %v", s.Username, err))
			outputs = append(outputs, s.Username+":\n"+outputTail(string(output), 10))
		}
	}

	if len(failures) > 0 {
		return "", strings.Join(outputs, "\n"), errors.New(strings.Join(failures, "; "))
	}
	if !wingetFound {
		return "", "", errUserWinGetNotFound
	}
	if !installed {
		return "", "", ErrPackageNotInstalled
	}
	if len(updated) == 0 {
		return "", "", ErrNoApplicableUpdate
	}

	log.Printf("[INFO]: winget.exe has upgraded the app %s for %s", packageID, strings.Join(updated, ", "))
	return "updated for " + strings.Join(updated, ", "), "", nil
}

var (
	errWinGetNoPackagesFound = errors.New("no packages found")
	errNoLoggedInUsers       = errors.New("user-scoped packages can only be updated while their user is logged in")
	errUserWinGetNotFound    = errors.New("winget.exe was not found in the sessions of the logged-in users")
)

// runWinGet runs winget.exe as SYSTEM, without a window and with a below normal priority
func runWinGet(timeout time.Duration, wgPath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, wgPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW | windows.BELOW_NORMAL_PRIORITY_CLASS,
	}

	output, err := cmd.CombinedOutput()
	return string(output), wingetError(err)
}

// wingetError translates winget's exit codes into readable errors
func wingetError(err error) error {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return err
	}

	code := fmt.Sprintf("0x%08X", uint32(exitErr.ExitCode()))
	switch code {
	case wingetNoApplicableUpdate:
		return ErrNoApplicableUpdate
	case wingetNoPackagesFound:
		return errWinGetNoPackagesFound
	}

	if message, ok := wingetcfg.ErrorCodes[code]; ok {
		return &wingetExitError{code: code, message: message}
	}
	return &wingetExitError{code: code, message: exitErr.Error()}
}

type wingetExitError struct {
	code    string
	message string
}

func (e *wingetExitError) Error() string {
	return fmt.Sprintf("winget failed with %s: %s", e.code, e.message)
}

func isExitError(err error) bool {
	var wgErr *wingetExitError
	return errors.As(err, &wgErr)
}

// ignoreNoUpdatesError discards the errors winget returns when there is nothing to upgrade
func ignoreNoUpdatesError(err error) error {
	if errors.Is(err, ErrNoApplicableUpdate) || errors.Is(err, errWinGetNoPackagesFound) {
		return nil
	}
	return err
}

// userWinGetPath returns the path to the winget.exe app execution alias of a user
func userWinGetPath(s runtime.UserSession) string {
	localAppData := s.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return ""
	}

	wgPath := filepath.Join(localAppData, "Microsoft", "WindowsApps", "winget.exe")
	// App execution aliases are reparse points, so Lstat is used instead of Stat
	if _, err := os.Lstat(wgPath); err != nil {
		return ""
	}
	return wgPath
}
