package vnc

import (
	"github.com/labstack/echo/v4"
)

type VNCServer struct {
	Name                 string
	StartCommand         string
	StartCommandArgs     []string
	StartCommandArgsFunc func(port string, xauthority string) []string
	StopCommand          string
	StopCommandArgs      []string
	KillCommand          string
	KillCommandArgs      []string
	Configure            func() (string, error)
	ConfigureAsUser      bool
	SavePIN              func(pin string) error
	RemovePIN            func() error
	Proxy                *echo.Echo
	ProxyCert            string
	ProxyKey             string
	ProxyPort            string
	ServerPort           string
}

func New(certPath, keyPath, sid, proxyPort string) (*VNCServer, error) {
	server, err := GetSupportedVNCServer(sid, proxyPort)
	if err != nil {
		return nil, err
	}
	server.Proxy = echo.New()
	server.ProxyCert = certPath
	server.ProxyKey = keyPath
	server.ProxyPort = proxyPort
	return server, nil
}
