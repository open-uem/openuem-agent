package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentFileURL(t *testing.T) {
	hash := strings.Repeat("a", 64)
	assert.True(t, isAgentFileURL("https://console.example.com:1324/agent-files/3/"+hash+"/app.msi"))

	for _, u := range []string{
		"http://console.example.com:1324/agent-files/3/" + hash + "/app.msi",
		"https://example.com/app.msi",
		"https://console.example.com:1324/agent-files/3/nothash/app.msi",
		`\\server\share\app.msi`,
		`C:\packages\app.msi`,
	} {
		assert.False(t, isAgentFileURL(u), u)
	}
}

func TestDownloadVerifiedFile(t *testing.T) {
	content := "msi content"
	sum := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(sum[:])

	var agentID string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentID = r.Header.Get(agentIDHeader)
		if strings.Contains(r.URL.Path, "forbidden") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	dir := t.TempDir()

	path, err := downloadVerifiedFile(server.Client(), server.URL+"/agent-files/1/"+hash+"/app.msi", "agent1", dir)
	require.NoError(t, err)
	assert.Equal(t, "agent1", agentID)
	assert.Equal(t, hash+".msi", path[len(dir)+1:])
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))

	// A file whose content doesn't match the hash in the URL is rejected
	_, err = downloadVerifiedFile(server.Client(), server.URL+"/agent-files/1/"+strings.Repeat("b", 64)+"/app.msi", "agent1", dir)
	assert.ErrorContains(t, err, "SHA-256")

	_, err = downloadVerifiedFile(server.Client(), server.URL+"/agent-files/forbidden/"+hash+"/app.msi", "agent1", dir)
	assert.ErrorContains(t, err, "403")

	// Rejected or corrupted downloads don't leave files behind
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestAgentFileExtension(t *testing.T) {
	hash := strings.Repeat("a", 64)
	assert.Equal(t, ".msi", agentFileExtension("https://c:1324/agent-files/1/"+hash+"/app.msi"))
	assert.Equal(t, ".msi", agentFileExtension("https://c:1324/agent-files/1/"+hash+"/app.msi?x=1#y"))
	assert.Equal(t, "", agentFileExtension("https://c:1324/agent-files/1/"+hash+"/app.ms'i"))
	assert.Equal(t, "", agentFileExtension("https://c:1324/agent-files/1/"+hash+"/app"))
}
