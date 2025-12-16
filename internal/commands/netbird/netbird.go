package netbird

import (
	"encoding/json"
	"log"

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

func RefreshInfo(data []byte) (*openuem_nats.Netbird, error) {
	return report.RetrieveNetbirdInfo()
}
