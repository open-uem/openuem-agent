package remotedesktop

import "github.com/labstack/echo/v4"

type RemoteDesktopService struct {
	Name                 string
	StartCommand         string
	SystemctlCommand     string
	StartCommandArgs     []string
	StartCommandArgsFunc func(username string, port string, xauthority string) []string
	StopCommand          string
	StopCommandArgs      []string
	StopCommandArgsFunc  func(username string) []string
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
	Username             string
	RequiresVNCProxy     bool
}

func New(certPath, keyPath, sid, proxyPort string) (*RemoteDesktopService, error) {
	agentOS := GetAgentOS()

	server, err := GetSupportedRemoteDesktopService(agentOS, sid, proxyPort)
	if err != nil {
		return nil, err
	}
	server.Proxy = echo.New()

	// Hide echo banners
	server.Proxy.HideBanner = true
	server.Proxy.HidePort = true

	server.ProxyCert = certPath
	server.ProxyKey = keyPath
	server.ProxyPort = proxyPort
	return server, nil
}
