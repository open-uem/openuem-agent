package agent

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	openuem_utils "github.com/open-uem/utils"
)

const (
	// agentFilesURLPrefix is the path where the console serves the files uploaded for tasks
	agentFilesURLPrefix = "/agent-files/"
	// agentIDHeader identifies the agent to the console, which only serves the files of
	// the profiles applied to it
	agentIDHeader = "X-OpenUEM-Agent"

	agentFileDownloadTimeout = 60 * time.Minute
)

var agentFileHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// agentFileHash returns the SHA-256 hash of a file uploaded to the console, taken from
// its URL https://<console>/agent-files/<profile>/<sha256>/<name>
func agentFileHash(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || !strings.HasPrefix(u.Path, agentFilesURLPrefix) {
		return "", false
	}

	parts := strings.Split(strings.TrimPrefix(u.Path, agentFilesURLPrefix), "/")
	if len(parts) != 3 || !agentFileHashPattern.MatchString(parts[1]) {
		return "", false
	}
	return parts[1], true
}

// isAgentFileURL reports if a task path is a file uploaded to the console
func isAgentFileURL(rawURL string) bool {
	_, ok := agentFileHash(rawURL)
	return ok
}

// downloadAgentFile downloads a file uploaded to the console for a task, authenticating
// with the agent certificate, and returns its local path. The caller removes the file.
func (a *Agent) downloadAgentFile(rawURL string) (string, error) {
	client, err := a.agentFilesClient()
	if err != nil {
		return "", err
	}

	dir, err := agentDataDir()
	if err != nil {
		return "", err
	}

	return downloadVerifiedFile(client, rawURL, a.Config.UUID, filepath.Join(dir, "downloads"))
}

// agentFilesClient is an HTTPS client that presents the agent certificate. It trusts
// OpenUEM's CA and the system's CAs, as a reverse proxy may use a public certificate.
func (a *Agent) agentFilesClient() (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(a.Config.AgentCert, a.Config.AgentKey)
	if err != nil {
		return nil, fmt.Errorf("could not read the agent certificate, reason: %v", err)
	}

	caCert, err := openuem_utils.ReadPEMCertificate(a.Config.CACert)
	if err != nil {
		return nil, fmt.Errorf("could not read the CA certificate, reason: %v", err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	pool.AddCert(caCert)

	return &http.Client{
		Timeout: agentFileDownloadTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				RootCAs:      pool,
				MinVersion:   tls.VersionTLS12,
			},
		},
	}, nil
}

// downloadVerifiedFile downloads a file uploaded to the console into dir and checks that
// its content matches the SHA-256 hash in its URL
func downloadVerifiedFile(client *http.Client, rawURL, agentID, dir string) (string, error) {
	hash, ok := agentFileHash(rawURL)
	if !ok {
		return "", errors.New("the URL is not a file uploaded to the console")
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set(agentIDHeader, agentID)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not download %s, reason: %v", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("could not download %s, the console answered %s", rawURL, resp.Status)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	// The file name comes from the hash and a plain extension, so it's safe to use in a
	// command line
	localPath := filepath.Join(dir, hash+agentFileExtension(rawURL))
	tmp, err := os.CreateTemp(dir, ".download-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())

	sum := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, sum), resp.Body); err != nil {
		tmp.Close()
		return "", fmt.Errorf("could not download %s, reason: %v", rawURL, err)
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	if got := hex.EncodeToString(sum.Sum(nil)); got != hash {
		return "", fmt.Errorf("the downloaded file has the SHA-256 hash %s instead of %s", got, hash)
	}

	if err := os.Rename(tmp.Name(), localPath); err != nil {
		return "", err
	}
	return localPath, nil
}

var agentFileExtensionPattern = regexp.MustCompile(`^\.[A-Za-z0-9]{1,8}$`)

// agentFileExtension returns the extension of the file in a URL, if it's a plain one
func agentFileExtension(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := path.Ext(u.Path)
	if !agentFileExtensionPattern.MatchString(ext) {
		return ""
	}
	return ext
}
