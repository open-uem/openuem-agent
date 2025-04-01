package logger

import "os"

type OpenUEMLogger struct {
	LogFile *os.File
}

func (l *OpenUEMLogger) Close() {
	l.LogFile.Close()
}
