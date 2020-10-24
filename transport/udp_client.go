package transport

import (
	"bytes"
	"io"
	"log"
	"net"
)

type UDPClient struct {
	addr      string
	writer    io.Writer
	conn      net.Conn
	closeChan chan struct{}
}

func NewUDPClient(addr string, w io.Writer) *UDPClient {
	return &UDPClient{
		addr:      addr,
		writer:    w,
		closeChan: make(chan struct{}, 1),
	}
}

func (c *UDPClient) RunFeedbackSender() (io.Writer, chan<- struct{}) {
	fbw := FeedbackWriter(make(chan []byte, 1024))
	done := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case fb := <-fbw:
				_, err := c.conn.Write(fb)
				if err != nil {
					log.Println(err)
				}
			case <-done:
				return
			}
		}
	}()
	return fbw, done
}

func (c *UDPClient) CloseChan() chan struct{} {
	return c.closeChan
}

func (c *UDPClient) Run() error {
	log.Println("running UDP Client")
	serverAddr, err := net.ResolveUDPAddr("udp", c.addr)
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return err
	}
	c.conn = conn

	_, err = conn.Write([]byte("hello"))
	if err != nil {
		return err
	}

	buf := make([]byte, 1500)
	for {
		select {
		case <-c.closeChan:
			return nil
		default:
		}
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			log.Println(err)
		}
		_, err = io.Copy(c.writer, bytes.NewReader(buf[:n]))
		if err != nil && err != io.EOF {
			return err
		}
	}
}
