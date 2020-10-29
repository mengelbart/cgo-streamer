package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"log"

	"github.com/lucas-clemente/quic-go"
)

type QUICClient struct {
	addr      string
	config    *quic.Config
	session   quic.Session
	writer    io.Writer
	closeChan chan struct{}
	dgram     bool
}

var tlsConf = &tls.Config{
	InsecureSkipVerify: true,
	NextProtos:         []string{"quic-realtime"},
}

func NewQUICClient(addr string, w io.Writer, dgram bool) *QUICClient {
	return &QUICClient{
		dgram: dgram,
		addr:  addr,
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

func (c *QUICClient) RunFeedbackSender() (io.Writer, chan<- struct{}, error) {
	fbw := FeedbackWriter(make(chan []byte, 1024))
	done := make(chan struct{}, 1)
	var fbSender func([]byte) error
	if c.dgram {
		fbSender = func(fb []byte) error {
			return c.session.SendMessage(fb)
		}
	} else {
		fbStream, err := c.session.OpenUniStreamSync(context.Background())
		if err != nil {
			return nil, nil, err
		}
		fbSender = func(fb []byte) error {
			_, err := fbStream.Write(fb)
			return err
		}
	}
	go func() {
		for {
			select {
			case fb := <-fbw:
				fbSender(fb)
			case <-done:
				return
			}
		}
	}()
	return fbw, done, nil
}

func (c *QUICClient) Run() error {
	if c.dgram {
		return c.RunDgram()
	}
	return c.RunStreamPerFrame()
}

const maxFlowControlWindow = uint64(1 << 60)

func (c *QUICClient) RunStreamPerFrame() error {
	log.Println("running streamperframe client")
	c.config.MaxReceiveStreamFlowControlWindow = maxFlowControlWindow
	c.config.MaxReceiveConnectionFlowControlWindow = maxFlowControlWindow
	session, err := quic.DialAddr(
		c.addr,
		tlsConf,
		c.config,
	)
	if err != nil {
		return err
	}
	c.session = session

	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}

	_, err = stream.Write([]byte("hello"))
	if err != nil {
		return err
	}

	for {
		select {
		case <-c.closeChan:
			return nil
		default:
		}
		stream, err := session.AcceptStream(context.Background())
		if err != nil {
			// TODO: Figure out correct error handling
			if err.Error() == "Application error 0x1: eos" {
				return nil
			}
			return err
		}
		bs, err := ioutil.ReadAll(stream)
		if err != nil {
			return err
		}
		_, err = io.Copy(c.writer, bytes.NewReader(bs))
		if err != nil && err != io.EOF {
			return err
		}
	}
}

func (c *QUICClient) RunDgram() error {
	log.Println("running dgram client")
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
			// TODO: Figure out correct error handling
			if err.Error() == "Application error 0x1: eos" {
				return nil
			}
			return err
		}
		_, err = io.Copy(c.writer, bytes.NewReader(bs))
		if err != nil && err != io.EOF {
			return err
		}
	}
}
