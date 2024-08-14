package icons

import (
	"os"
	"path/filepath"
)

func Data() (*[]byte, error) {
	path, _ := os.Getwd()
	icon, err := os.ReadFile(filepath.Join(path, "assets", "icons", "openuem-circle.ico"))
	if err != nil {
		return nil, err
	}
	return &icon, nil
}
