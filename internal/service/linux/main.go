//go:build linux

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/open-uem/openuem-agent/internal/logger"
)

func main() {
	// Instantiate logger
	l := logger.New()

	// Instantiate service
	s := NewService(l)

	s.Execute()

	// TODO LINUX Keep the connection alive for service
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-done
}
