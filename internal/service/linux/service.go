//go:build linux

package main

import (
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
}
