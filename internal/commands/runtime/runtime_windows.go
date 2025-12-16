//go:build windows

package runtime

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// Reference: https://blog.davidvassallo.me/2022/06/17/golang-in-windows-execute-command-as-another-user/
func getUserToken(pid int) (syscall.Token, error) {
	var err error
	var token syscall.Token

	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		log.Printf("[ERROR]: token process, reason: %v\n", err)
	}
	defer func() {
		if err := syscall.CloseHandle(handle); err != nil {
			log.Println("[ERROR]: could not close user token handle")
		}

	}()

	// Find process token via win32
	err = syscall.OpenProcessToken(handle, syscall.TOKEN_ALL_ACCESS, &token)

	if err != nil {
		log.Printf("[ERROR]: open token process, reason: %v\n", err)
	}
	return token, err
}

const processEntrySize = 568

// Reference: https://stackoverflow.com/questions/36333896/how-to-get-process-id-by-process-name-in-windows-environment
func findProcessByName(name string) (uint32, error) {
	h, e := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if e != nil {
		return 0, e
	}
	p := windows.ProcessEntry32{Size: processEntrySize}
	for {
		e := windows.Process32Next(h, &p)
		if e != nil {
			return 0, e
		}
		if windows.UTF16ToString(p.ExeFile[:]) == name {
			return p.ProcessID, nil
		}
	}
}

// Reference: https://blog.davidvassallo.me/2022/06/17/golang-in-windows-execute-command-as-another-user/
func RunAsUser(cmdPath string, args []string) error {
	pid, err := findProcessByName("explorer.exe")
	if err != nil {
		return err
	}

	token, err := getUserToken(int(pid))
	if err != nil {
		return err
	}
	defer token.Close()

	cmd := exec.Command(cmdPath, args...)

	// this is the important bit!
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token:         token,
		CreationFlags: 0x08000000, // Reference: https://stackoverflow.com/questions/42500570/how-to-hide-command-prompt-window-when-using-exec-in-golang
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	err = SetPriorityWindows(cmd.Process.Pid, windows.IDLE_PRIORITY_CLASS)
	if err != nil {
		log.Println("[ERROR]: could not change process priority")
	}

	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}

// Reference: https://blog.davidvassallo.me/2022/06/17/golang-in-windows-execute-command-as-another-user/
func RunAsUserWithOutput(cmdPath string, args []string) ([]byte, error) {
	pid, err := findProcessByName("explorer.exe")
	if err != nil {
		return nil, err
	}

	token, err := getUserToken(int(pid))
	if err != nil {
		return nil, err
	}
	defer token.Close()

	cmd := exec.Command(cmdPath, args...)

	// this is the important bit!
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token:         token,
		CreationFlags: 0x08000000, // Reference: https://stackoverflow.com/questions/42500570/how-to-hide-command-prompt-window-when-using-exec-in-golang
	}

	if cmd.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	err = SetPriorityWindows(cmd.Process.Pid, windows.IDLE_PRIORITY_CLASS)
	if err != nil {
		log.Println("[ERROR]: could not change process priority")
	}

	err = cmd.Wait()
	if err != nil {
		return nil, err
	}

	return stdout.Bytes(), err
}

func RunAsUserWithOutputAndTimeout(cmdPath string, args []string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdPath, args...)

	pid, err := findProcessByName("explorer.exe")
	if err != nil {
		return nil, err
	}

	token, err := getUserToken(int(pid))
	if err != nil {
		return nil, err
	}
	defer token.Close()

	// this is the important bit!
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token:         token,
		CreationFlags: 0x08000000, // Reference: https://stackoverflow.com/questions/42500570/how-to-hide-command-prompt-window-when-using-exec-in-golang
	}

	if cmd.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	err = SetPriorityWindows(cmd.Process.Pid, windows.IDLE_PRIORITY_CLASS)
	if err != nil {
		log.Println("[ERROR]: could not change process priority")
	}

	err = cmd.Wait()
	if err != nil {
		return nil, err
	}

	return stdout.Bytes(), err
}

const PROCESS_ALL_ACCESS = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0xffff

func SetPriorityWindows(pid int, priority uint32) error {
	handle, err := windows.OpenProcess(PROCESS_ALL_ACCESS, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle) // Technically this can fail, but we ignore it

	err = windows.SetPriorityClass(handle, priority)
	if err != nil {
		return err
	}

	return nil
}

// Reference: https://blog.davidvassallo.me/2022/06/17/golang-in-windows-execute-command-as-another-user/
func RunAsUserInBackground(cmdPath string, args []string) error {
	pid, err := findProcessByName("explorer.exe")
	if err != nil {
		return err
	}

	token, err := getUserToken(int(pid))
	if err != nil {
		return err
	}
	defer token.Close()

	cmd := exec.Command(cmdPath, args...)

	// this is the important bit!
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token:         token,
		CreationFlags: 0x08000000, // Reference: https://stackoverflow.com/questions/42500570/how-to-hide-command-prompt-window-when-using-exec-in-golang
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	return nil
}
