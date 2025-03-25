package vnc

import (
	"log"
	"net"
	"net/http"
	"time"

	"github.com/evangwt/go-vncproxy"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
)

type VNCServer struct {
	Name            string
	StartCommand    string
	StopCommand     string
	StopCommandArgs []string
	KillCommand     string
	KillCommandArgs []string
	Configure       func() error
	ConfigureAsUser bool
	SavePIN         func(pin string) error
	Proxy           *echo.Echo
	ProxyCert       string
	ProxyKey        string
	ProxyPort       string
}

func New(certPath, keyPath, sid, proxyPort string) (*VNCServer, error) {
	server, err := GetSupportedVNCServer(sid)
	if err != nil {
		return nil, err
	}
	server.Proxy = echo.New()
	server.ProxyCert = certPath
	server.ProxyKey = keyPath
	server.ProxyPort = proxyPort
	return server, nil
}

func (vnc *VNCServer) StartProxy() {
	log.Printf("[INFO]: starting VNC proxy on port %s\n", vnc.ProxyPort)
	// Launch proxy only if port is available
	_, err := net.DialTimeout("tcp", ":"+vnc.ProxyPort, 5*time.Second)
	if err != nil {
		vncProxy := vncproxy.New(&vncproxy.Config{
			LogLevel: vncproxy.InfoFlag,
			TokenHandler: func(r *http.Request) (addr string, err error) {
				return ":5900", nil
			},
		})
		vnc.Proxy.GET("/ws", func(ctx echo.Context) error {
			h := websocket.Handler(vncProxy.ServeWS)
			h.ServeHTTP(ctx.Response().Writer, ctx.Request())
			return nil
		})

		log.Println("[INFO]: NoVNC proxy server started")

		if err := vnc.Proxy.StartTLS(":"+vnc.ProxyPort, vnc.ProxyCert, vnc.ProxyKey); err != http.ErrServerClosed {
			log.Printf("[ERROR]: could not start VNC proxy\n, %v", err)
		}

	} else {
		log.Printf("[ERROR]: VNC proxy port %s is not available\n", vnc.ProxyPort)
	}
}
