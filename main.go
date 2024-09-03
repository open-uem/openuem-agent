package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/doncicuto/openuem-agent/internal/agent"
	"github.com/doncicuto/openuem-agent/internal/utils"
	"golang.org/x/sys/windows/svc"
)

type openUEMService struct{}

func (m openUEMService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	// Create Agent and Start it
	a := agent.Agent{}
	a.Start()

	// service control manager
loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Println("stop or shutdown...")
				break loop
			default:
				log.Println("unexpected control request")
				return true, 1
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return true, 0
}

/*
	 func init() {

		a := agent.Agent{}
		a.Test()
		go a.Start()

}
*/
func main() {

	// Instantiate custom logger
	customLogger()
	s := openUEMService{}
	err := svc.Run("openuem-agent", s)
	if err != nil {
		log.Fatalf("could not run service: %v", err)
	}
}

func customLogger() {
	wd, err := utils.Getwd()
	if err != nil {
		log.Fatalf("could not get cwd: %v", err)
	}

	logPath := filepath.Join(wd, "logs", "openuem-log.txt")
	f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("could not create log file: %v", err)
	}
	/* defer f.Close() */

	log.SetOutput(f)
	log.SetPrefix("openuem-agent: ")
	log.SetFlags(log.Ldate | log.Ltime)
}
