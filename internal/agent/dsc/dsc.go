//go:build windows

package dsc

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	"golang.org/x/sys/windows"
)

type TaskControl struct {
	Success         []string             `json:"success"`
	Executed        map[string]time.Time `json:"executed"`
	ProfilesRunning map[string]time.Time `json:"profilesRunning"`
}

func ReadTaskControlFile(taskControl string) (*TaskControl, error) {
	if _, err := os.Stat(taskControl); err != nil {
		f, err := os.Create(taskControl)
		defer func() {
			if err := f.Close(); err != nil {
				log.Printf("[ERROR]: could not close the task control file, reason: %v", err)
			}
		}()
		if err != nil {
			log.Printf("[ERROR]: could not create the task control file, reason: %v", err)
			return nil, err
		}

		t := TaskControl{}
		data, err := json.Marshal(t)
		if err != nil {
			log.Printf("[ERROR]: could not marshall initial task control, reason: %v", err)
		}
		if _, err := f.Write(data); err != nil {
			log.Printf("[ERROR]: could not write initial data to task control file, reason: %v", err)
		}
		return &t, nil
	} else {
		data, err := os.ReadFile(taskControl)
		if err != nil {
			log.Printf("[ERROR]: could not read the task control file, reason: %v", err)
			return nil, err
		}
		t := TaskControl{}
		if err := json.Unmarshal(data, &t); err != nil {
			log.Printf("[ERROR]: could not unmarshall JSON data from the task control file, reason: %v", err)
			return nil, err
		}

		return &t, nil
	}
}

func SetTaskAsSuccessfull(taskID string, taskControlPath string, t *TaskControl) error {

	taskAlreadySuccessful := slices.Contains(t.Success, taskID)
	if taskAlreadySuccessful {
		return nil
	}

	t.Success = append(t.Success, taskID)

	out, err := json.Marshal(t)
	if err != nil {
		log.Printf("[ERROR]: could not marshal JSON data for the task control file, reason: %v", err)
		return err
	}

	if err := os.WriteFile(taskControlPath, out, 0660); err != nil {
		log.Printf("[ERROR]: could not write JSON data to the task control file, reason: %v", err)
		return err
	}

	return nil
}

func SaveTaskControl(taskControlPath string, t *TaskControl) error {
	out, err := json.Marshal(t)
	if err != nil {
		log.Printf("[ERROR]: could not marshal JSON data for the task control file, reason: %v", err)
		return err
	}

	if err := os.WriteFile(taskControlPath, out, 0660); err != nil {
		log.Printf("[ERROR]: could not write executed task as JSON data to the task control file, reason: %v", err)
		return err
	}

	return nil
}

func SetProfileAsRunning(taskControlPath string, t *TaskControl) error {
	out, err := json.Marshal(t)
	if err != nil {
		log.Printf("[ERROR]: could not marshal JSON data for the task control file, reason: %v", err)
		return err
	}

	if err := os.WriteFile(taskControlPath, out, 0660); err != nil {
		log.Printf("[ERROR]: could not write executed task as JSON data to the task control file, reason: %v", err)
		return err
	}

	return nil
}

func RunTaskWithLowPriority(command string) error {
	var out bytes.Buffer

	cmd := exec.Command("PowerShell", "-command", command)
	cmd.Stderr = &out

	err := cmd.Start()
	if err != nil {
		return err
	}

	err = runtime.SetPriorityWindows(cmd.Process.Pid, windows.IDLE_PRIORITY_CLASS)
	if err != nil {
		log.Println("[ERROR]: could not change process priority")
		return err
	}

	err = cmd.Wait()
	if err != nil {
		errMessages := strings.Split(out.String(), ".")
		return errors.New(errMessages[0])
	}

	return nil
}
