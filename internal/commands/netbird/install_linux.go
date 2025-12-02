//go:build linux

package netbird

import (
	"log"
	"os"
	"os/exec"
)

func Install() error {
	var err error

	c1 := exec.Command("curl", "-fsSL", "https://pkgs.netbird.io/install.sh")
	c2 := exec.Command("sh")
	c2.Stdin, err = c1.StdoutPipe()
	if err != nil {
		log.Printf("[ERROR]: could not create the pipe for the curl install NetBird command, reason: %v", err)
		return err
	}
	c2.Stdout = os.Stdout
	if err := c2.Start(); err != nil {
		log.Printf("[ERROR]: could not start the sh command for NetBird install command, reason: %v", err)
		return err
	}
	if err := c1.Run(); err != nil {
		log.Printf("[ERROR]: could not download the NetBird install script, reason: %v", err)
		return err
	}
	if err := c2.Wait(); err != nil {
		log.Printf("[ERROR]: could not install the NetBird client, reason: %v", err)
		return err
	}
	return nil
}
