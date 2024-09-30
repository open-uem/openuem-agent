package icons

import (
	"os"
	"path/filepath"

	"github.com/doncicuto/openuem-agent/internal/agent"
)

func Data() (*[]byte, error) {
	path, err := agent.Getwd()
	if err != nil {
		return nil, err
	}

	icon, err := os.ReadFile(filepath.Join(path, "assets", "openuem.ico"))
	if err != nil {
		return nil, err
	}
	return &icon, nil
}
