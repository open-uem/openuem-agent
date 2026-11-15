//go:build windows

package main

import (
	"log"
	"runtime"
	"time"

	"github.com/open-uem/openuem-agent/internal/logger"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func main() {

	// the agent will use two CPUs at maximum
	runtime.GOMAXPROCS(2)

	// Instantiate logger
	l := logger.New()

	// Instantiate service
	s := NewService(l)

	// Ask the SCM to restart the service if the agent process dies
	setRecoveryActions()

	// Run service
	err := svc.Run("openuem-agent", s)
	if err != nil {
		log.Fatalf("could not run service: %v", err)
	}
}

// setRecoveryActions configures the service control manager to restart the
// agent when it terminates without reporting SERVICE_STOPPED (e.g., when the
// process dies during sleep/hibernate transitions)
func setRecoveryActions() {
	m, err := mgr.Connect()
	if err != nil {
		log.Printf("[ERROR]: could not connect to the service manager, reason: %v", err)
		return
	}
	defer m.Disconnect()

	s, err := m.OpenService("openuem-agent")
	if err != nil {
		log.Printf("[ERROR]: could not open the openuem-agent service, reason: %v", err)
		return
	}
	defer s.Close()

	// Restart after 1, 1 and 5 minutes and reset the failure count every day
	if err := s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: time.Minute},
		{Type: mgr.ServiceRestart, Delay: time.Minute},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Minute},
	}, 24*60*60); err != nil {
		log.Printf("[ERROR]: could not set service recovery actions, reason: %v", err)
	}
}
