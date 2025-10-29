//go:build windows

package main

import (
	"log"
	"runtime"

	"github.com/open-uem/openuem-agent/internal/logger"
	"golang.org/x/sys/windows/svc"
)

func main() {

	// the agent will use two CPUs at maximum
	runtime.GOMAXPROCS(2)

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
