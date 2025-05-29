//go:build darwin

package main

import (
	"github.com/open-uem/openuem-agent/internal/logger"
)

func main() {
	// Instantiate logger
	l := logger.New()

	// Instantiate service
	s := NewService(l)

	s.Execute()
}
