package logger

import (
	"log"
	"os"
	"path/filepath"
)

type OpenUEMLogger struct {
	LogFile *os.File
}

func New() *OpenUEMLogger {
	logger := OpenUEMLogger{}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("could not get cwd: %v", err)
	}

	logPath := filepath.Join(wd, "logs", "openuem-log.txt")
	logger.LogFile, err = os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("could not create log file: %v", err)
	}

	log.SetOutput(logger.LogFile)
	log.SetPrefix("openuem-agent: ")
	log.SetFlags(log.Ldate | log.Ltime)

	return &logger
}

func (l *OpenUEMLogger) Close() {
	l.LogFile.Close()
}
