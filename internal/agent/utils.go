package agent

import (
	"os"
	"path/filepath"
)

func Getwd() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(ex), nil
}
