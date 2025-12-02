package netbird

import (
	"encoding/json"
	"log"

	"github.com/nats-io/nats.go"
	openuem_nats "github.com/open-uem/nats"
)

func Respond(msg *nats.Msg, errMessage string) {
	result := openuem_nats.RustDeskResult{
		Error: errMessage,
	}

	data, err := json.Marshal(result)
	if err != nil {
		log.Printf("[ERROR]: could not marshal NetBird action response, reason: %v\n", err)
	}

	if err := msg.Respond(data); err != nil {
		log.Printf("[ERROR]: could not respond to NetBird action message, reason: %v\n", err)
		return
	}
}
