//go:build darwin

package logger

import (
	"log"
	"os"
	"path/filepath"
)

func New() *OpenUEMLogger {
	var err error

	logger := OpenUEMLogger{}

	wd := "/var/log/openuem-agent"

	if _, err := os.Stat(wd); os.IsNotExist(err) {
		if err := os.MkdirAll(wd, 0660); err != nil {
			log.Fatalf("[FATAL]: could not create log directory, reason: %v", err)
		}
	}

	logFilename := "openuem-agent.log"
	logPath := filepath.Join(wd, logFilename)

	logger.LogFile, err = os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("could not create log file: %v", err)
	}

	log.SetOutput(logger.LogFile)
	log.SetPrefix("openuem-agent: ")
	log.SetFlags(log.Ldate | log.Ltime)

	return &logger
}
