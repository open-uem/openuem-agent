//go:build windows

package dsc

import (
	"bytes"
	"errors"
	"log"
	"os/exec"
	"strings"

	"github.com/open-uem/openuem-agent/internal/commands/runtime"
	"golang.org/x/sys/windows"
)

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
