//go:build windows

package remotedesktop

import (
	"log"
	"os/exec"
	"syscall"

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

	err = cmd.Wait()
	if err != nil {
		return err
	}
	return nil
}
