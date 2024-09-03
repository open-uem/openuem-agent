package agent

import (
	"log"

	"github.com/nats-io/nats.go"
)

func (a *Agent) connect() error {

	natsURL := a.Config.ServerUrl
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	// Create NATS connection
	c, err := nats.Connect(
		natsURL,
		nats.MaxReconnects(-1),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Println("[INFO]: Reconnected to the message broker")
		}),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				log.Printf("[INFO]: Disconnected from message broker due to: %s, will attempt reconnect", err)
			}
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Printf("[ERROR]: Connection closed. Reason: %q\n", nc.LastError())
		}),
	)
	if err != nil {
		return err
	}
	a.NatsConnection = c
	return nil
}
