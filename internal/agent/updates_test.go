package agent

import (
	"errors"
	"path/filepath"
	"testing"

	openuem_nats "github.com/open-uem/nats"
	"github.com/stretchr/testify/assert"
)

func TestUpdateRunTracker(t *testing.T) {
	path := filepath.Join(t.TempDir(), updateRunFilename)
	tracker := loadUpdateRunTracker(path)
	assert.Nil(t, tracker.lastRun())

	assert.True(t, tracker.start(0))
	assert.False(t, tracker.start(1), "a second run can't start while the first is running")

	tracker.setTotal(2)
	tracker.addResult(openuem_nats.UpdateResult{AvailableUpdate: openuem_nats.AvailableUpdate{PackageId: "7zip.7zip"}})

	run := tracker.lastRun()
	assert.True(t, run.Running())
	assert.Equal(t, 2, run.Total)
	assert.Len(t, run.Results, 1)

	// The returned run is a copy
	run.Results[0].Failed = true
	assert.False(t, tracker.lastRun().Results[0].Failed)

	// A run that was in progress when the agent stopped is reported as interrupted
	restarted := loadUpdateRunTracker(path)
	assert.False(t, restarted.lastRun().Running())
	assert.NotEmpty(t, restarted.lastRun().Error)
	assert.Len(t, restarted.lastRun().Results, 1)

	tracker.finish(errors.New("boom"))
	assert.False(t, tracker.lastRun().Running())
	assert.Equal(t, "boom", tracker.lastRun().Error)
	assert.True(t, tracker.start(0))
}
