package transport

import (
	"bytes"
	"crypto/tls"
	"io"

	"github.com/lucas-clemente/quic-go"
)

type QUICClient struct {
	addr      string
	config    *quic.Config
	session   quic.Session
	writer    io.Writer
	closeChan chan struct{}
}

var tlsConf = &tls.Config{
	InsecureSkipVerify: true,
	NextProtos:         []string{"quic-realtime"},
}

func NewQUICClient(addr string, w io.Writer) *QUICClient {
	return &QUICClient{
		addr: addr,
		config: &quic.Config{
			MaxIncomingStreams:    maxStreamCount,
			MaxIncomingUniStreams: maxStreamCount,
		},
		writer:    w,
		closeChan: make(chan struct{}, 1),
	}
}

type FeedbackWriter chan []byte

func (f FeedbackWriter) Write(b []byte) (int, error) {
	f <- b
	return len(b), nil
}

func (c *QUICClient) CloseChan() chan struct{} {
	return c.closeChan
}

func (c *QUICClient) RunFeedbackSender() (io.Writer, chan<- struct{}) {
	fbw := FeedbackWriter(make(chan []byte, 1024))
	done := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case fb := <-fbw:
				c.session.SendMessage(fb)
			case <-done:
				return
			}
		}
	}()
	return fbw, done
}

func (c *QUICClient) Run() error {
	c.config.EnableDatagrams = true
	session, err := quic.DialAddr(
		c.addr,
		tlsConf,
		c.config,
	)
	if err != nil {
		return err
	}
	c.session = session

	for {
		select {
		case <-c.closeChan:
			return nil
		default:
		}
		bs, err := c.session.ReceiveMessage()
		if err != nil {
			panic(err)
		}
		_, err = io.Copy(c.writer, bytes.NewReader(bs))
		if err != nil && err != io.EOF {
			return err
		}
	}
}
