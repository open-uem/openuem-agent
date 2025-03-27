//go:build linux

package main

import (
	"log"

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
}
