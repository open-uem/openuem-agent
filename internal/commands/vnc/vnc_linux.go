//go:build linux

package vnc

import "fmt"

// TODO LINUX - Start VNC Server
func (vnc *VNCServer) Start(pin string, notifyUser bool) {
}

// TODO LINUX - Stop VNC Server
func (vnc *VNCServer) Stop() {
}

// TODO LINUX - Get supported VNC servers
func GetSupportedVNCServer(sid string) (*VNCServer, error) {
	return nil, fmt.Errorf("no supported VNC server")
}
