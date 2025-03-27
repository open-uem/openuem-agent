//go:build linux

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/open-uem/openuem-agent/internal/commands/report"
	"github.com/open-uem/openuem-agent/internal/logger"
)

func main() {
	// Instantiate logger
	l := logger.New()

	// Instantiate service
	s := NewService(l)

	s.Execute()

	r, err := report.RunReport("", true, false, "", "", "")
	if err != nil {
		log.Println("error running report")
	}
	r.Print()

	// TODO LINUX Keep the connection alive
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-done
}
