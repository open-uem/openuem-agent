package agent

import (
	"log"

	"github.com/go-co-op/gocron/v2"
)

func (a *Agent) startScheduler() error {
	var err error
	a.TaskScheduler, err = gocron.NewScheduler()
	if err != nil {
		log.Fatalf("[FATAL]: could not create the scheduler: %v", err)
		return err
	}

	// Start scheduler
	a.TaskScheduler.Start()
	log.Println("[INFO]: task scheduler has started!")
	return nil
}

func (a *Agent) stopScheduler() error {
	if a.TaskScheduler != nil {
		if err := a.TaskScheduler.Shutdown(); err != nil {
			log.Printf("[ERROR]: there was an error trying to shutdown the task scheduler %v", err)
			return err
		}
	}

	log.Println("[INFO]: task scheduler has been shutdown")
	return nil
}
