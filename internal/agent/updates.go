package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/deploy"
)

const (
	updateRunFilename = "last_update_run.json"

	// Queued update requests older than this are ignored, e.g. for a computer that
	// was switched off for months
	queuedUpdateRequestMaxAge = 30 * 24 * time.Hour
	// A queued request that arrives while a run is in progress is delivered again later
	queuedUpdateRequestRetryDelay = 10 * time.Minute
)

// packageUpdateLock prevents running several package manager updates at the same time,
// e.g. a single package update requested while a bulk update run is in progress
var packageUpdateLock sync.Mutex

// updateRunTracker keeps the state of the last bulk update run. It's saved to a JSON
// file so the result of the last run survives agent restarts.
type updateRunTracker struct {
	mu   sync.Mutex
	run  *openuem_nats.UpdateRun
	path string
}

func newUpdateRunTracker() *updateRunTracker {
	dir, err := agentDataDir()
	if err != nil {
		log.Printf("[ERROR]: could not get the agent's data directory, the last update run won't be saved, reason: %v", err)
		return &updateRunTracker{}
	}
	return loadUpdateRunTracker(filepath.Join(dir, updateRunFilename))
}

// loadUpdateRunTracker creates a tracker that saves its state in path, reading the last run if any
func loadUpdateRunTracker(path string) *updateRunTracker {
	t := &updateRunTracker{path: path}

	data, err := os.ReadFile(t.path)
	if err != nil {
		return t
	}

	run := openuem_nats.UpdateRun{}
	if err := json.Unmarshal(data, &run); err != nil {
		log.Printf("[ERROR]: could not read the last update run, reason: %v", err)
		return t
	}

	// The agent was stopped while the run was in progress
	if run.Running() {
		run.Finished = time.Now()
		run.Error = "the update run was interrupted"
	}
	t.run = &run

	return t
}

// start begins a new run unless one is already in progress
func (t *updateRunTracker) start(total int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.run.Running() {
		return false
	}

	t.run = &openuem_nats.UpdateRun{Started: time.Now(), Total: total}
	t.save()
	return true
}

func (t *updateRunTracker) setTotal(total int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.run.Total = total
	t.save()
}

func (t *updateRunTracker) addResult(result openuem_nats.UpdateResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.run.Results = append(t.run.Results, result)
	t.save()
}

func (t *updateRunTracker) finish(runErr error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.run.Finished = time.Now()
	if runErr != nil {
		t.run.Error = runErr.Error()
	}
	t.save()
}

// lastRun returns a copy of the last run, or nil if no run has been performed
func (t *updateRunTracker) lastRun() *openuem_nats.UpdateRun {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.run == nil {
		return nil
	}
	run := *t.run
	run.Results = append([]openuem_nats.UpdateResult{}, t.run.Results...)
	return &run
}

// save must be called with the lock held
func (t *updateRunTracker) save() {
	if t.path == "" {
		return
	}

	data, err := json.Marshal(t.run)
	if err != nil {
		log.Printf("[ERROR]: could not marshal the update run, reason: %v", err)
		return
	}

	if err := os.WriteFile(t.path, data, 0600); err != nil {
		log.Printf("[ERROR]: could not save the update run, reason: %v", err)
	}
}

// ListPackageUpdatesSubscribe answers with the packages that can be updated and the
// result of the last bulk update run. While a run is in progress, only its progress is sent.
func (a *Agent) ListPackageUpdatesSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.listpackageupdates."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		response := openuem_nats.AvailableUpdatesResponse{LastRun: a.updates.lastRun()}

		request := openuem_nats.ListPackageUpdatesRequest{}
		if len(msg.Data) > 0 {
			if err := json.Unmarshal(msg.Data, &request); err != nil {
				log.Printf("[ERROR]: could not unmarshal list package updates request, reason: %v", err)
			}
		}

		if !request.StatusOnly && !response.LastRun.Running() {
			updates, err := deploy.ListAvailableUpdates()
			if err != nil {
				log.Printf("[ERROR]: could not list available package updates, reason: %v", err)
				response.Error = err.Error()
			}
			response.Updates = updates
		}

		data, err := json.Marshal(response)
		if err != nil {
			log.Printf("[ERROR]: could not marshal available package updates, reason: %v", err)
			return
		}

		if err := msg.Respond(data); err != nil {
			log.Printf("[ERROR]: could not respond to list package updates message, reason: %v", err)
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent list package updates, reason: %v", err)
	}
	return nil
}

// UpdatePackagesSubscribe starts a bulk update run. The request is answered as soon as
// the run starts, with an empty message, or with the reason why it couldn't be started.
func (a *Agent) UpdatePackagesSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.updatepackages."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		request := openuem_nats.UpdatePackagesRequest{}
		if err := json.Unmarshal(msg.Data, &request); err != nil {
			log.Printf("[ERROR]: could not unmarshal update packages request, reason: %v", err)
			respondUpdatePackages(msg, "invalid update request")
			return
		}

		if !a.updates.start(len(request.Packages)) {
			respondUpdatePackages(msg, "an update run is already in progress")
			return
		}

		respondUpdatePackages(msg, "")
		go a.runPackageUpdates(request.Packages)
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent update packages, reason: %v", err)
	}
	return nil
}

// QueuedUpdatePackagesHandler starts an update run requested through JetStream, so computers
// that were offline when the run was requested start it when they connect again
func (a *Agent) QueuedUpdatePackagesHandler(msg jetstream.Msg) {
	request := openuem_nats.UpdatePackagesRequest{}
	if err := json.Unmarshal(msg.Data(), &request); err != nil {
		log.Printf("[ERROR]: could not unmarshal queued update packages request, reason: %v", err)
		ackQueuedMessage(msg)
		return
	}

	if !request.RequestedAt.IsZero() && time.Since(request.RequestedAt) > queuedUpdateRequestMaxAge {
		log.Printf("[INFO]: ignoring the update run requested on %s, it's too old", request.RequestedAt.Format(time.RFC3339))
		ackQueuedMessage(msg)
		return
	}

	// A run that started after the request covers it, e.g. when the update was requested
	// several times while the computer was offline
	if lastRun := a.updates.lastRun(); lastRun != nil && !request.RequestedAt.IsZero() && lastRun.Started.After(request.RequestedAt) {
		log.Printf("[INFO]: ignoring the update run requested on %s, a later run already started", request.RequestedAt.Format(time.RFC3339))
		ackQueuedMessage(msg)
		return
	}

	if !a.updates.start(len(request.Packages)) {
		log.Println("[INFO]: an update run is already in progress, the queued update run will be retried later")
		if err := msg.NakWithDelay(queuedUpdateRequestRetryDelay); err != nil {
			log.Printf("[ERROR]: could not NAK message, reason: %v", err)
		}
		return
	}

	ackQueuedMessage(msg)
	go a.runPackageUpdates(request.Packages)
}

func ackQueuedMessage(msg jetstream.Msg) {
	if err := msg.Ack(); err != nil {
		log.Printf("[ERROR]: could not ACK message, reason: %v", err)
	}
}

func respondUpdatePackages(msg *nats.Msg, reason string) {
	if err := msg.Respond([]byte(reason)); err != nil {
		log.Printf("[ERROR]: could not respond to update packages message, reason: %v", err)
	}
}

// runPackageUpdates updates the packages one after another. If no package is given, every
// package with an available update is updated. Each result is sent to the worker too, so
// deployments made with OpenUEM keep their status.
func (a *Agent) runPackageUpdates(packages []openuem_nats.AvailableUpdate) {
	if len(packages) == 0 {
		var err error
		packages, err = deploy.ListAvailableUpdates()
		if err != nil {
			log.Printf("[ERROR]: could not list available package updates, reason: %v", err)
			a.updates.finish(err)
			return
		}
		a.updates.setTotal(len(packages))
	}

	log.Printf("[INFO]: starting update run for %d packages", len(packages))

	for _, p := range packages {
		result := a.updatePackage(p)
		a.updates.addResult(result)
	}

	a.updates.finish(nil)
	log.Println("[INFO]: update run has finished")

	// Send a report to update the installed apps
	r := a.RunReport()
	if r == nil {
		return
	}

	if err := a.SendReport(r); err != nil {
		log.Printf("[ERROR]: report could not be send to NATS server!, reason: %s\n", err.Error())
	}
}

func (a *Agent) updatePackage(p openuem_nats.AvailableUpdate) openuem_nats.UpdateResult {
	result := openuem_nats.UpdateResult{AvailableUpdate: p}

	action := openuem_nats.DeployAction{
		AgentId:         a.Config.UUID,
		Action:          "update",
		PackageId:       p.PackageId,
		PackageName:     p.PackageName,
		PackageBranch:   p.PackageBranch,
		PackageBrewType: p.PackageBrewType,
		PackageScope:    p.Scope,
	}

	if !deploy.IsValidPackageID(p.PackageId) {
		result.Failed = true
		result.Info = "invalid package id"
		return result
	}

	packageUpdateLock.Lock()
	_, info, err := deploy.UpdatePackage(action)
	packageUpdateLock.Unlock()

	switch {
	case err == nil:
	case errors.Is(err, deploy.ErrNoApplicableUpdate):
		result.UpToDate = true
		return result
	case errors.Is(err, deploy.ErrPackageNotInstalled):
		result.UpToDate = true
		result.Info = err.Error()
		return result
	default:
		result.Failed = true
		result.Info = strings.TrimSpace(err.Error() + "\n" + info)
	}

	action.When = time.Now()
	action.Failed = result.Failed
	action.Info = result.Info
	if err := a.SendDeployResult(&action); err != nil {
		log.Printf("[ERROR]: could not send deploy result to worker, reason: %v\n", err)
	}

	return result
}

// agentDataDir is the directory where the agent keeps its state files
func agentDataDir() (string, error) {
	if runtime.GOOS == "linux" {
		return "/etc/openuem-agent", nil
	}
	return Getwd()
}
