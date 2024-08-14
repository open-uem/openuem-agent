package main

import (
	"github.com/doncicuto/openuem-agent/internal/agent"
	"github.com/doncicuto/openuem-agent/internal/log"
)

func main() {
	// instantiate logger
	log.NewLogger()
	defer log.CloseLogger()

	// start agent
	a := agent.Agent{}
	a.Start()
}
