//go:build windows

package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"time"

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

func (a *Agent) RebootSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.reboot."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		log.Println("[INFO]: reboot request received")
		if err := msg.Respond([]byte("Reboot!")); err != nil {
			log.Printf("[ERROR]: could not respond to agent reboot message, reason: %v\n", err)
		}

		action := openuem_nats.RebootOrRestart{}
		if err := json.Unmarshal(msg.Data, &action); err != nil {
			log.Printf("[ERROR]: could not unmarshal to agent reboot message, reason: %v\n", err)
			return
		}

		when := int(time.Until(action.Date).Seconds())
		if when > 0 {
			if err := exec.Command("cmd", "/C", "shutdown", "/r", "/t", strconv.Itoa(when)).Run(); err != nil {
				fmt.Printf("[ERROR]: could not initiate power off, reason: %v", err)
			}
		} else {
			if err := exec.Command("cmd", "/C", "shutdown", "/r").Run(); err != nil {
				fmt.Printf("[ERROR]: could not initiate power off, reason: %v", err)
			}
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent reboot, reason: %v", err)
	}
	return nil
}

func (a *Agent) PowerOffSubscribe() error {
	_, err := a.NATSConnection.QueueSubscribe("agent.poweroff."+a.Config.UUID, "openuem-agent-management", func(msg *nats.Msg) {
		log.Println("[INFO]: power off request received")
		if err := msg.Respond([]byte("Power Off!")); err != nil {
			log.Printf("[ERROR]: could not respond to agent power off message, reason: %v\n", err)
			return
		}

		action := openuem_nats.RebootOrRestart{}
		if err := json.Unmarshal(msg.Data, &action); err != nil {
			log.Printf("[ERROR]: could not unmarshal to agent power off message, reason: %v\n", err)
			return
		}

		when := int(time.Until(action.Date).Seconds())
		if when > 0 {
			if err := exec.Command("cmd", "/C", "shutdown", "/s", "/t", strconv.Itoa(when)).Run(); err != nil {
				fmt.Printf("[ERROR]: could not initiate power off, reason: %v", err)
			}
		} else {
			if err := exec.Command("cmd", "/C", "shutdown", "/s").Run(); err != nil {
				fmt.Printf("[ERROR]: could not initiate shutdown, reason: %v", err)
			}
		}
	})

	if err != nil {
		return fmt.Errorf("[ERROR]: could not subscribe to agent power off, reason: %v", err)
	}
	return nil
}
