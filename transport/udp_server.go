package transport

import (
	"log"
	"net"
)

type PacketHandler interface {
	handle(conn net.PacketConn, addr net.Addr, buf []byte) error
}

type UDPServer struct {
	PacketHandler
	addr string
}

func NewUDPServer(addr string, options ...func(*UDPServer)) *UDPServer {
	s := &UDPServer{
		addr: addr,
	}
	for _, option := range options {
		option(s)
	}
	return s
}

func SetPacketHandler(ph PacketHandler) func(*UDPServer) {
	return func(s *UDPServer) {
		s.PacketHandler = ph
	}
}

func (s *UDPServer) Run() error {
	log.Println("running UDP server")
	pc, err := net.ListenPacket("udp", s.addr)
	if err != nil {
		return err
	}
	return s.accept(pc)
}

func (s *UDPServer) accept(conn net.PacketConn) error {
	for {
		buf := make([]byte, 1500)
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("Could not read from connection: %v\n", err)
		}
		go func() {
			var err error
			defer func() {
				if err != nil {
					log.Printf("connection error: %v\n", err)
				}
			}()
			err = s.handle(conn, addr, buf[:n])
		}()
	}
}
