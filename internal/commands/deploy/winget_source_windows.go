//go:build windows

package deploy

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const wingetSourceTimeout = 5 * time.Minute

// RegisterWinGetSource registers a WinGet REST source for SYSTEM's winget, which the agent
// uses to install, update and uninstall packages. An existing source with the same name is
// replaced. The source is added as trusted (see wingetSourceAddArgs). With trustCA, the CA certificate is added to the machine's trusted roots first,
// as winget only accepts sources with a trusted certificate.
func RegisterWinGetSource(name, sourceURL, caCertPath string, trustCA bool) error {
	if !strings.HasPrefix(sourceURL, "https://") || !IsValidPackageID(name) {
		return fmt.Errorf("invalid winget source %q", name)
	}

	if trustCA {
		if out, err := exec.Command(systemTool("certutil.exe"), "-addstore", "-f", "Root", caCertPath).CombinedOutput(); err != nil {
			return fmt.Errorf("could not trust the CA certificate, reason: %v, %s", err, strings.TrimSpace(string(out)))
		}
	}

	wgPath, err := locateWinGet()
	if err != nil {
		return fmt.Errorf("could not locate winget.exe, reason: %v", err)
	}

	// The source may exist with another URL, e.g. after the console's address changed
	_, _ = runWinGet(wingetSourceTimeout, wgPath, "source", "remove", "--name", name, "--disable-interactivity")

	started := time.Now()
	output, err := runWinGet(wingetSourceTimeout, wgPath, wingetSourceAddArgs(name, sourceURL)...)
	if err != nil {
		reason := fmt.Sprintf("could not add the winget source, reason: %v, %s", err, outputTail(output, 10))
		if logFile, lines := wingetLogExcerpt(started); len(lines) > 0 {
			reason += fmt.Sprintf("\nwinget's log (%s):\n  %s", logFile, strings.Join(lines, "\n  "))
		}
		return fmt.Errorf("%s", reason)
	}
	return nil
}

// wingetLogExcerpt returns the newest winget log written since a time, and the lines of it
// that explain a failure. winget run by SYSTEM writes its logs to SYSTEM's temp folder or,
// depending on how it's started, to the App Installer's folder in SYSTEM's profile.
func wingetLogExcerpt(since time.Time) (string, []string) {
	roots := []string{
		filepath.Join(os.TempDir(), "WinGet"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Packages", "Microsoft.DesktopAppInstaller_8wekyb3d8bbwe", "LocalState", "DiagOutputDir"),
		filepath.Join(os.Getenv("SystemRoot"), "SystemTemp", "WinGet"),
		filepath.Join(os.Getenv("SystemRoot"), "Temp", "WinGet"),
	}

	newest, newestTime := "", since.Add(-time.Second)
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasPrefix(d.Name(), "WinGet-") || !strings.HasSuffix(d.Name(), ".log") {
				return nil
			}
			if info, err := d.Info(); err == nil && info.ModTime().After(newestTime) {
				newest, newestTime = path, info.ModTime()
			}
			return nil
		})
	}
	if newest == "" {
		return "", nil
	}

	data, err := readTail(newest, 1<<20)
	if err != nil {
		return newest, nil
	}
	return newest, FilterWinGetLog(string(data), 15)
}

// systemTool returns the path of a program in System32, so the service's PATH isn't searched
func systemTool(name string) string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32", name)
}

// readTail reads at most the last max bytes of a file
func readTail(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	offset := info.Size() - max
	if offset < 0 {
		offset = 0
	}
	data := make([]byte, info.Size()-offset)
	_, err = f.ReadAt(data, offset)
	if err == io.EOF {
		err = nil
	}
	return data, err
}
