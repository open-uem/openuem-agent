package log

import (
	"log"
	"os"
)

var (
	f      *os.File
	Logger *log.Logger
)

func NewLogger() {
	var err error
	f, err = os.OpenFile("openuem-log.txt", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("could not create logger instance: %v", err)
	}

	Logger = log.New(f, "openuem-agent: ", log.Ldate|log.Ltime)
}

func CloseLogger() {
	f.Close()
}
