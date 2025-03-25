//go:build linux

package logger

import (
	"log"
	"os"
)

func New() *OpenUEMLogger {
	var err error

	logger := OpenUEMLogger{}

	logPath := "/var/log/openuem-agent"
	logger.LogFile, err = os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("could not create log file: %v", err)
	}

	log.SetOutput(logger.LogFile)
	log.SetPrefix("openuem-agent: ")
	log.SetFlags(log.Ldate | log.Ltime)

	return &logger
}
