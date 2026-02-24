//go:build windows

package dsc

import (
	"bytes"
	"log"
	"os/exec"

	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	"golang.org/x/sys/windows"
)

func RunTaskWithLowPriority(command string) (string, string, error) {
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer

	cmd := exec.Command("PowerShell", "-command", command)
	cmd.Stderr = &stdErr
	cmd.Stdout = &stdOut

	err := cmd.Start()
	if err != nil {
		return "", "", err
	}

	err = runtime.SetPriorityWindows(cmd.Process.Pid, windows.IDLE_PRIORITY_CLASS)
	if err != nil {
		log.Println("[ERROR]: could not change process priority")
		return "", "", err
	}

	err = cmd.Wait()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return "", "", err
		}
	}

	return stdOut.String(), stdErr.String(), nil
}
