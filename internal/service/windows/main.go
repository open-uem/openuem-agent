package main

import (
	"log"

	"github.com/open-uem/openuem-agent/internal/logger"
	"golang.org/x/sys/windows/svc"
)

func main() {
	// Instantiate logger
	l := logger.New()

	// Instantiate service
	s := NewService(l)

	// Run service
	err := svc.Run("openuem-agent", s)
	if err != nil {
		log.Fatalf("could not run service: %v", err)
	}
}
