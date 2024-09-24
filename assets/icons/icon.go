package icons

import (
	"os"
	"path/filepath"
)

func Data() (*[]byte, error) {
	path, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	icon, err := os.ReadFile(filepath.Join(path, "assets", "openuem.ico"))
	if err != nil {
		return nil, err
	}
	return &icon, nil
}
