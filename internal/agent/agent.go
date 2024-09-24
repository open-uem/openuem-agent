package agent

import (
	"log"
	"time"

	"github.com/doncicuto/openuem_nats"
	"github.com/go-co-op/gocron/v2"
)

type Agent struct {
	Config         Config
	TaskScheduler  gocron.Scheduler
	AgentJob       gocron.Job
	NATSConnectJob gocron.Job
	MessageServer  *openuem_nats.MessageServer
}

func New() Agent {
	var err error
	agent := Agent{}

	agent.TaskScheduler, err = gocron.NewScheduler()
	if err != nil {
		log.Fatalf("[FATAL]: could not create the scheduler: %v", err)
	}

	agent.ReadConfig()

	/* agent.MessageServer = openuem_nats.New() */

	return agent
}

func (a *Agent) Start() {
	if a.Config.UUID == "" {
		a.SetInitialConfig()
	}

	/* else {
		a.ID = a.Config.UUID
		a.FirstContact = a.Config.FirstExecutionDate
		a.LastContact = time.Now()
	} */

	// Start task scheduler
	a.TaskScheduler.Start()
	log.Println("[INFO]: task scheduler has started!")

	// Try to connect to NATS server and start a reconnect job if failed
	if err := a.MessageServer.Connect(); err != nil {
		a.startNATSConnectJob()
		return
	}

	// Run agent if all is ready
	/* 	if a.NatsConnection != nil {
		a.Run(true)
		a.startAgentJob()
	} */
}

func (a *Agent) Run(force bool) {
	start := time.Now()

	log.Println("[INFO]: agent is running...")

	// TODO - maybe send report if some important properties have changed (like antivirus, windows update..)
	/* 	if !a.Config.DidIReportToday() || force {
	   		// Get the information
	   		a.getInfoFromOS()

	   		a.LastContact = start
	   		a.Enabled = a.Config.Enabled

	   		// Create JSON
	   		data, err := json.Marshal(a.Report)
	   		if err != nil {
	   			log.Printf("[ERROR]: could not marshal report data %v\n", err)
	   		}

	   		// Send NATS information
	   		if a.NatsConnection != nil {
	   			// TODO - set timeout for waiting for NATS process as a constant
	   			if _, err := a.NatsConnection.Request("update", data, 5*time.Minute); err != nil {
	   				log.Printf("[ERROR]: could not sent report to server %v\n", err)
	   			} else {
	   				a.Config.LastReportDate = start
	   				log.Println("[INFO]: report was sent to server!")
	   			}
	   		} else {
	   			log.Printf("[ERROR]: no connection with server is available %v\n", err)
	   		}
	   	} else {
	   		log.Println("[INFO]: agent has already reported today skip sending to server")
	   	}
	*/
	log.Printf("[INFO]: agent execution took %v\n", time.Since(start))

	a.Config.LastExecutionDate = start
	a.Config.WriteConfig()
}

func (a *Agent) startAgentJob() error {
	var err error
	// Create task for running the agent
	a.AgentJob, err = a.TaskScheduler.NewJob(
		gocron.DurationJob(
			time.Duration(time.Duration(a.Config.ExecuteEveryXMinutes)*time.Minute),
		),
		gocron.NewTask(
			func() {
				a.Run(false)
			},
		),
	)
	if err != nil {
		log.Fatalf("[FATAL]: could not start the agent job: %v", err)
		return err
	}
	log.Printf("[INFO]: new agent job has been scheduled every %d minutes", a.Config.ExecuteEveryXMinutes)
	return nil
}

func (a *Agent) startNATSConnectJob() error {
	var err error
	// Create task for running the agent
	a.NATSConnectJob, err = a.TaskScheduler.NewJob(
		gocron.DurationJob(
			time.Duration(5*time.Minute),
		),
		gocron.NewTask(
			func() {
				err := a.MessageServer.Connect()
				if err == nil {
					a.TaskScheduler.RemoveJob(a.NATSConnectJob.ID())
				}
			},
		),
	)
	if err != nil {
		log.Fatalf("[FATAL]: could not start the NATS connect job: %v", err)
		return err
	}
	log.Printf("[INFO]: new NATS connect job has been scheduled every %d minutes", a.Config.ExecuteEveryXMinutes)
	return nil
}
