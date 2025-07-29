package rustdesk

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/nats-io/nats.go"
	openuem_nats "github.com/open-uem/nats"
)

type RustDeskOptionsEntries struct {
	CustomRendezVousServer string `toml:"custom-rendezvous-server"`
	RelayServer            string `toml:"relay-server"`
	Key                    string `toml:"key"`
	ApiServer              string `toml:"api-server"`
	UseDirectIPAccess      string `toml:"direct-server,omitempty"`
	Whitelist              string `toml:"whitelist,omitempty"`
}

type RustDeskOptions struct {
	Optional RustDeskOptionsEntries `toml:"options"`
}

type RustDeskPassword struct {
	Password string `toml:"password"`
	Salt     string `toml:"salt"`
}

// Reference: https://stackoverflow.com/questions/73864379/golang-change-permission-os-chmod-and-os-chowm-recursively
func ChownRecursively(root string, uid int, gid int) error {

	return filepath.Walk(root,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			err = os.Chown(path, uid, gid)
			if err != nil {
				return err
			}
			return nil
		})
}

// Reference: https://leapcell.io/blog/how-to-copy-a-file-in-go
func CopyFile(src, dst string) error {
	// Open the source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destinationFile.Close()

	// Copy the content
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Flush file metadata to disk
	err = destinationFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	return nil
}

func RustDeskRespond(msg *nats.Msg, id string, errMessage string) {
	result := openuem_nats.RustDeskResult{
		RustDeskID: id,
		Error:      errMessage,
	}

	data, err := json.Marshal(result)
	if err != nil {
		log.Printf("[ERROR]: could not marshal RustDesk response, reason: %v\n", err)
	}

	if err := msg.Respond(data); err != nil {
		log.Printf("[ERROR]: could not respond to agent rustdesk start message, reason: %v\n", err)
		return
	}
}
