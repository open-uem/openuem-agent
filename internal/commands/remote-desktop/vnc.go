package remotedesktop

import (
	"log"
	"net"
	"net/http"
	"time"

	"github.com/evangwt/go-vncproxy"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
)

type RemoteDesktopService struct {
	Name             string
	StartCommand     string
	SystemctlCommand string
	StopCommand      string
	StopCommandArgs  []string
	Configure        func() error
	SavePIN          func(pin string) error
	RemovePIN        func() error
	Proxy            *echo.Echo
	ProxyCert        string
	ProxyKey         string
	ProxyPort        string
	RequiresVNCProxy bool
	StartService     func(vncPort string) error
	StopService      func() error
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

func (rd *RemoteDesktopService) StartVNCProxy(port string) {
	log.Printf("[INFO]: starting VNC proxy on port %s\n", rd.ProxyPort)
	// Launch proxy only if port is available
	_, err := net.DialTimeout("tcp", ":"+rd.ProxyPort, 5*time.Second)
	if err != nil {
		vncProxy := vncproxy.New(&vncproxy.Config{
			LogLevel: vncproxy.InfoFlag,
			TokenHandler: func(r *http.Request) (addr string, err error) {
				if port != "" {
					return ":" + port, nil
				}
				return ":5900", nil
			},
		})
		rd.Proxy.GET("/ws", func(ctx echo.Context) error {
			h := websocket.Handler(vncProxy.ServeWS)
			h.ServeHTTP(ctx.Response().Writer, ctx.Request())
			return nil
		})

		log.Println("[INFO]: NoVNC proxy server started")

		if err := rd.Proxy.StartTLS(":"+rd.ProxyPort, rd.ProxyCert, rd.ProxyKey); err != http.ErrServerClosed {
			log.Printf("[ERROR]: could not start VNC proxy\n, %v", err)
		}

	} else {
		log.Printf("[ERROR]: VNC proxy port %s is not available\n", rd.ProxyPort)
	}
}
