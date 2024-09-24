package main

import (
	"log"

	"github.com/doncicuto/openuem-agent/internal/logger"
	"github.com/doncicuto/openuem-agent/internal/service"
	"golang.org/x/sys/windows/svc"
)

func main() {
	// Instantiate logger
	l := logger.New()

	// Instantiate service
	s := service.New(l)

	// Run service
	err := svc.Run("openuem-agent", s)
	if err != nil {
		log.Fatalf("could not run service: %v", err)
	}
}
