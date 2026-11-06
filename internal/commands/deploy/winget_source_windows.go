//go:build windows

package deploy

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const wingetSourceTimeout = 5 * time.Minute

// RegisterWinGetSource registers a WinGet REST source for SYSTEM's winget, which the agent
// uses to install, update and uninstall packages. An existing source with the same name is
// replaced. With trustCA, the CA certificate is added to the machine's trusted roots first,
// as winget only accepts sources with a trusted certificate.
func RegisterWinGetSource(name, sourceURL, caCertPath string, trustCA bool) error {
	if !strings.HasPrefix(sourceURL, "https://") || !IsValidPackageID(name) {
		return fmt.Errorf("invalid winget source %q", name)
	}

	if trustCA {
		if out, err := exec.Command("certutil.exe", "-addstore", "-f", "Root", caCertPath).CombinedOutput(); err != nil {
			return fmt.Errorf("could not trust the CA certificate, reason: %v, %s", err, strings.TrimSpace(string(out)))
		}
	}

	wgPath, err := locateWinGet()
	if err != nil {
		return fmt.Errorf("could not locate winget.exe, reason: %v", err)
	}

	// The source may exist with another URL, e.g. after the console's address changed
	_, _ = runWinGet(wingetSourceTimeout, wgPath, "source", "remove", "--name", name, "--disable-interactivity")

	output, err := runWinGet(wingetSourceTimeout, wgPath, "source", "add", "--name", name, "--arg", sourceURL,
		"--type", "Microsoft.Rest", "--accept-source-agreements", "--disable-interactivity")
	if err != nil {
		return fmt.Errorf("could not add the winget source, reason: %v, %s", err, outputTail(output, 10))
	}
	return nil
}
