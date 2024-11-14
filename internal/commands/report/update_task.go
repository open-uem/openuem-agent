package report

import (
	"time"

	"github.com/doncicuto/openuem_utils"
	"golang.org/x/sys/windows/registry"
)

func (r *Report) getUpdateTaskInfo() error {
	k, err := openuem_utils.OpenRegistryForQuery(registry.LOCAL_MACHINE, `SOFTWARE\OpenUEM\Agent`)
	if err != nil {
		return err
	}
	defer func() {
		k.Close()
	}()

	executionTime, err := openuem_utils.GetValueFromRegistry(k, "UpdaterLastExecutionTime")
	if err != nil {
		return err
	}

	if executionTime != "" {
		r.AgentReport.LastUpdateTaskExecutionTime, err = time.ParseInLocation("2006-01-02T15:04:05", executionTime, time.Local)
		if err != nil {
			return err
		}
	}

	r.AgentReport.LastUpdateTaskStatus, err = openuem_utils.GetValueFromRegistry(k, "UpdaterLastExecutionStatus")
	if err != nil {
		return err
	}

	r.AgentReport.LastUpdateTaskResult, err = openuem_utils.GetValueFromRegistry(k, "UpdaterLastExecutionResult")
	if err != nil {
		return err
	}

	return nil
}
