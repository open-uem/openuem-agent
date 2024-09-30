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

	// Get executable path to store logs
	ex, err := os.Executable()
	if err != nil {
		log.Fatalf("could not get executable info: %v", err)
	}
	wd := filepath.Dir(ex)

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
