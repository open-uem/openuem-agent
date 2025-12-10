package netbird

import (
	"encoding/json"
	"log"
	"os/exec"

	"github.com/nats-io/nats.go"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/report"
)

func Respond(msg *nats.Msg, n *openuem_nats.Netbird) {
	data, err := json.Marshal(n)
	if err != nil {
		log.Printf("[ERROR]: could not marshal NetBird action response, reason: %v\n", err)
	}

	if err := msg.Respond(data); err != nil {
		log.Printf("[ERROR]: could not respond to NetBird action message, reason: %v\n", err)
		return
	}
}

func Register(data []byte) (*openuem_nats.Netbird, error) {
	request := openuem_nats.NetbirdRegister{}
	if err := json.Unmarshal(data, &request); err != nil {
		log.Printf("[ERROR]: could not unmarshal the NetBird register request, reason: %v", err)
		return nil, err
	}

	bin := getNetbirdBin()

	// First, we must set the connection down
	if err := exec.Command(bin, "down").Run(); err != nil {
		log.Println("[ERROR]: could not execute netbird down")
		return nil, err
	}

	// Now, use the key and URL to register the agent
	if err := exec.Command(bin, "up", "--setup-key", request.OneOffKey, "--management-url", request.ManagementURL).Run(); err != nil {
		log.Println("[ERROR]: could not execute netbird up")
		return nil, err
	}

	return report.RetrieveNetbirdInfo()
}
