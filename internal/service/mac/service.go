//go:build darwin

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/open-uem/openuem-agent/internal/agent"
	"github.com/open-uem/openuem-agent/internal/logger"
)

type OpenUEMService struct {
	Logger *logger.OpenUEMLogger
}

func NewService(l *logger.OpenUEMLogger) *OpenUEMService {
	return &OpenUEMService{
		Logger: l,
	}
}

func (s *OpenUEMService) Execute() {
	// Get new agent
	a := agent.New()

	// Start agent
	a.Start()

	// Keep the connection alive for service
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-done

	// Stop agent
	log.Println("[INFO]: service has received the stop or shutdown command")
	s.Logger.Close()
	a.Stop()
}
