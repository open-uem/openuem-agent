package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/doncicuto/openuem-agent/assets/icons"
	"github.com/doncicuto/openuem-agent/internal/log"
	"github.com/getlantern/systray"
	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/scjalliance/comshim"
	"golang.org/x/sys/windows"
)

type Report struct {
	ID             string    `json:"id,omitempty"`
	OS             string    `json:"os,omitempty"`
	Hostname       string    `json:"hostname,omitempty"`
	Version        string    `json:"version,omitempty"`
	InitialContact time.Time `json:"initial_contact,omitempty"`
	LastContact    time.Time `json:"last_contact,omitempty"`
	Edges          Edges     `json:"edges"`
}

type Agent struct {
	Report
	Config         Config
	TaskScheduler  gocron.Scheduler
	NatsConnection *nats.Conn
}

type Edges struct {
	Computer        Computer         `json:"computer,omitempty"`
	Antivirus       Antivirus        `json:"antivirus,omitempty"`
	OperatingSystem OperatingSystem  `json:"operatingsystem,omitempty"`
	LogicalDisks    []LogicalDisk    `json:"logicaldisks,omitempty"`
	Monitors        []Monitor        `json:"monitors,omitempty"`
	Printers        []Printer        `json:"printers,omitempty"`
	Shares          []Share          `json:"shares,omitempty"`
	SystemUpdate    SystemUpdate     `json:"systemupdate,omitempty"`
	NetworkAdapters []NetworkAdapter `json:"networkadapters,omitempty"`
	Applications    []Application    `json:"applications,omitempty"`
	LoggedOnUsers   []LoggedOnUser   `json:"loggedonusers,omitempty"`
}

func (a *Agent) Run(force bool) {
	start := time.Now()

	if !a.Config.didIReportToday() || force {
		log.Logger.Println("[INFO]: agent is running...")

		// Get the information
		a.getInfoFromOS()

		now := time.Now()
		a.LastContact = now

		// Create JSON
		data, err := json.Marshal(a.Report)
		if err != nil {
			log.Logger.Printf("[ERROR]: could not marshal report data %v\n", err)
		}

		// Send NATS information
		if err := a.NatsConnection.Publish("update", data); err != nil {
			log.Logger.Printf("[ERROR]: could not sent report to server %v\n", err)
		} else {
			// TODO - What happens if there's no subscribers for that subject
			a.Config.LastReportDate = now
			log.Logger.Println("[INFO]: report was sent to server!")
		}

		// Print the information to stdout
		a.printAgent()
	} else {
		log.Logger.Println("[SKIP]: agent has already reported today")
	}

	log.Logger.Printf("[INFO]: agent execution took %v\n", time.Since(start))

	a.Config.LastExecutionDate = start
	writeConfig(a.Config)
}

func (a *Agent) getInfoFromOS() {
	// Prepare COM
	comshim.Add(1)
	defer comshim.Done()

	a.Version = "0.0.1-alpha"
	a.OS = "windows"
	computerName, err := windows.ComputerName()
	if err == nil {
		a.Hostname = computerName
	}

	// These operations don't benefit from goroutines
	a.getComputerInfo()
	a.getOSInfo()
	a.getLogicalDisksInfo()
	a.getMonitorsInfo()
	a.getPrintersInfo()
	a.getSharesInfo()

	if !a.Edges.OperatingSystem.isWindowsServer() {
		a.getAntivirusInfo()
	}

	a.getSystemUpdateInfo()
	a.getNetworkAdaptersInfo()
	a.getApplicationsInfo()
}

func (a *Agent) printAgent() {
	fmt.Printf("\n** 🕵  Agent *********************************************************************************************************\n")
	fmt.Printf("%-40s |  %s\n", "Computer Name", a.Hostname)
	fmt.Printf("%-40s |  %s\n", "Version", a.Version)
	fmt.Printf("%-40s |  %s\n", "Agent ID", a.ID)
	fmt.Printf("%-40s |  %s\n", "Operating System", a.OS)
	fmt.Printf("%-40s |  %s\n", "Last report", a.LastContact)

	a.logComputer()
	a.logOS()
	a.logLogicalDisks()
	a.logMonitors()
	a.logPrinters()
	a.logShares()
	if !a.Edges.OperatingSystem.isWindowsServer() {
		a.logAntivirus()
	}
	a.logSystemUpdate()
	a.logNetworkAdapters()
	a.logApplications()
}

func (a *Agent) Start() {
	// Read config from JSON
	a.Config = readConfig()
	if a.Config.UUID == "" {
		id := uuid.New()
		a.Config.UUID = id.String()
		a.Config.ExecuteEveryXMinutes = 2
		a.ID = id.String()
		a.Config.FirstExecutionDate = time.Now()
		writeConfig(a.Config)
	} else {
		a.ID = a.Config.UUID
	}

	log.Logger.Println("[INFO]: application has started...")
	systray.Run(a.onReady, a.OnQuit)
}

func (a *Agent) onReady() {
	// Agent launches and try to add menu icon to systray
	// Credits: https://owenmoore.hashnode.dev/build-tray-gui-desktop-application-go

	icon, err := icons.Data()
	if err != nil {
		log.Logger.Fatalf("[FATAL]: icon could not be added to systray: %v", err)
	}
	systray.SetIcon(*icon)

	mRun := systray.AddMenuItem("Run Inventory", "Run Inventory and report it to OpenUEM server")
	mQuit := systray.AddMenuItem("Quit", "Quit OpenUEM")

	// Create NATS connection
	a.NatsConnection, err = nats.Connect(
		nats.DefaultURL,
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Logger.Printf("Disconnected due to: %s, will attempt reconnect", err)
			}
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Logger.Printf("[ERROR]: Connection closed. Reason: %q\n", nc.LastError())
		}),
	)
	if err != nil {
		log.Logger.Fatalf("[FATAL]: could not connect with server: %v\n", err)
	}
	log.Logger.Println("[INFO]: connection established with NATS server")

	// Subscribe to receive trigger to run agent
	a.NatsConnection.Subscribe(fmt.Sprintf("trigger-%s", a.ID), func(m *nats.Msg) {
		if string(m.Data) == a.ID {
			log.Logger.Println("[INFO]: a report has been triggered from OpenUEM server")
			a.Run(true)
		} else {
			log.Logger.Printf("[ERROR]: received wrong message from NATS %s\n", m.Data)
		}
	})

	// Run at agent start
	a.Run(true)

	// Schedule task to run agent every X minutes according to config
	a.startScheduler()

	// Wait for user actions on systray
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM, syscall.SIGINT)
	for {
		select {
		case <-mRun.ClickedCh:
			log.Logger.Println("[INFO]: user force a run of the OpenUEM Agent")
			force := true
			a.Run(force)
		case <-mQuit.ClickedCh:
			systray.Quit()
		case <-sigc:
			systray.Quit()
		}
	}
}

func (a *Agent) OnQuit() {
	a.stopScheduler()
	a.NatsConnection.Close()
	log.Logger.Println("[INFO]: agent has exited")
}

func (a *Agent) startScheduler() {
	var err error
	a.TaskScheduler, err = gocron.NewScheduler()
	if err != nil {
		log.Logger.Fatalf("[FATAL]: could not create the scheduler: %v", err)
	}

	// Get task duration from config
	var taskEveryMinutes uint8 = 5
	if a.Config.ExecuteEveryXMinutes > 0 {
		taskEveryMinutes = a.Config.ExecuteEveryXMinutes
	}
	taskDuration := time.Duration(taskEveryMinutes) * time.Minute

	// Create new job
	_, err = a.TaskScheduler.NewJob(
		gocron.DurationJob(
			time.Duration(taskDuration),
		),
		gocron.NewTask(
			func() {
				a.Run(false)
			},
		),
	)
	if err != nil {
		log.Logger.Fatalf("[FATAL]: could not start the scheduler: %v", err)
	} else {
		log.Logger.Printf("[INFO]: new job has been scheduled every %d minutes", taskEveryMinutes)
	}

	// Start scheduler
	a.TaskScheduler.Start()
	log.Logger.Println("[INFO]: task scheduler has started!")
}

func (a *Agent) stopScheduler() {
	if err := a.TaskScheduler.Shutdown(); err != nil {
		log.Logger.Printf("[ERROR]: there was an error trying to shutdown the task scheduler %v", err)
	} else {
		log.Logger.Println("[INFO]: task scheduler has been shutdown")
	}
}
