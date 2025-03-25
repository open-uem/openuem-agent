//go:build windows

package agent

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
	openuem_nats "github.com/open-uem/nats"
	"github.com/open-uem/openuem-agent/internal/commands/report"
	"github.com/open-uem/openuem-agent/internal/commands/vnc"
)

func (a *Agent) StartVNCSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.startvnc."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {

		loggedOnUser, err := report.GetLoggedOnUsername()
		if err != nil {
			log.Println("[ERROR]: could not get logged on username")
			return
		}

		sid, err := report.GetSID(loggedOnUser)
		if err != nil {
			log.Println("[ERROR]: could not get SID for logged on user")
			return
		}

		// Instantiate new vnc server, but first try to check if certificates are there
		a.GetServerCertificate()
		if a.ServerCertPath == "" || a.ServerKeyPath == "" {
			log.Println("[ERROR]: VNC requires a server certificate that it's not ready")
			return
		}

		v, err := vnc.New(a.ServerCertPath, a.ServerKeyPath, sid, a.Config.VNCProxyPort)
		if err != nil {
			log.Println("[ERROR]: could not get a VNC server")
			return
		}

		// Unmarshal data
		var vncConn openuem_nats.VNCConnection
		if err := json.Unmarshal(msg.Data, &vncConn); err != nil {
			log.Println("[ERROR]: could not unmarshall VNC connection")
			return
		}

		// Start VNC server
		a.VNCServer = v
		v.Start(vncConn.PIN, vncConn.NotifyUser)

		if err := msg.Respond([]byte("VNC Started!")); err != nil {
			log.Printf("[ERROR]: could not respond to agent start vnc message, reason: %v\n", err)
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent start vnc, reason: %v", err)
	}
	return nil
}
