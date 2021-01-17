package transport

import (
	"log"
	"net"
	"sync"
)

type UDPPacketHandler struct {
	src        SrcFactory
	sessions   map[string]*UDPPacketSession
	sessionMux sync.Mutex
}

func NewUDPPacketHandler(src SrcFactory) *UDPPacketHandler {
	return &UDPPacketHandler{
		src:      src,
		sessions: make(map[string]*UDPPacketSession),
	}
}

func (h *UDPPacketHandler) handle(conn net.PacketConn, addr net.Addr, buf []byte) error {
	h.sessionMux.Lock()
	defer h.sessionMux.Unlock()
	ps, ok := h.sessions[addr.String()]

	if !ok {
		ps = &UDPPacketSession{
			conn:     conn,
			addr:     addr,
			feedback: make(chan []byte, 1024),
		}
		cancel := h.src.MakeSrc(ps, ps.feedback, nil) // nil: QUIC Feedback not implemented for UDP
		ps.cancelFn = cancel
		h.sessions[addr.String()] = ps
		return nil
	}

	ps.AcceptFeedback(buf)
	return nil
}

type UDPPacketSession struct {
	conn     net.PacketConn
	addr     net.Addr
	feedback chan []byte
	cancelFn func()
}

func (s *UDPPacketSession) Close() error {
	log.Println("closing udp session")
	_, err := s.conn.WriteTo([]byte("eos"), s.addr)
	s.cancelFn()
	return err
}

func (s *UDPPacketSession) AcceptFeedback(msg []byte) {
	s.feedback <- msg
}

func (s *UDPPacketSession) Write(p []byte) (int, error) {
	return s.conn.WriteTo(p, s.addr)
}
