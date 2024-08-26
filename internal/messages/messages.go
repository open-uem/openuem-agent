package messages

import (
	"os"

	"github.com/doncicuto/openuem-agent/internal/log"
	"github.com/nats-io/nats.go"
)

func Connect() (*nats.Conn, error) {

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	// Create NATS connection
	c, err := nats.Connect(
		natsURL,
		nats.MaxReconnects(-1),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Logger.Println("[INFO]: Reconnected to the message broker")
		}),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Logger.Printf("[INFO]: Disconnected from message broker due to: %s, will attempt reconnect", err)
			}
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Logger.Printf("[ERROR]: Connection closed. Reason: %q\n", nc.LastError())
		}),
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}
