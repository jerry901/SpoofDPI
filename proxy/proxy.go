package proxy

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/xvzc/SpoofDPI/net"
	"github.com/xvzc/SpoofDPI/packet"
)

type Proxy struct {
	port string
}

func New(port string) *Proxy {
	return &Proxy{
		port: port,
	}
}

func (p *Proxy) Port() string {
	return p.port
}

func (p *Proxy) Start() {
	l, err := net.Listen("tcp", ":"+p.Port())
	if err != nil {
		log.Fatal("Error creating listener: ", err)
		os.Exit(1)
	}

	log.Println("Created a listener on :", p.Port())

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("Error accepting connection: ", err)
			continue
		}

		log.Debug("[PROXY] Accepted a new connection from ", conn.RemoteAddr())

		go func() {
			b, err := conn.ReadBytes()
			if err != nil {
				return
			}
			// log.Debug("[PROXY] Client sent a request")

			pkt, err := packet.NewHttpPacket(b)
            if err != nil {
                log.Debug("Error while parsing request: ", string(b))
                return
            }

			if !pkt.IsValidMethod() {
				log.Debug("Unsupported method: ", pkt.Method())
				return
			}

			if pkt.IsConnectMethod() {
				log.Debug("[HTTPS] Start")
				conn.HandleHttps(pkt)
			} else {
				log.Debug("[HTTP] Start")
				conn.HandleHttp(pkt)
			}
		}()
	}
}
