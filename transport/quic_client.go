package transport

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/mengelbart/cgo-streamer/util"

	"github.com/lucas-clemente/quic-go/logging"
	"github.com/lucas-clemente/quic-go/qlog"

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

func NewQUICClient(addr string, w io.Writer, dgram bool, qlogFile string) *QUICClient {
	qc := &QUICClient{
		dgram: dgram,
		addr:  addr,
		config: &quic.Config{
			MaxIncomingStreams:    maxStreamCount,
			MaxIncomingUniStreams: maxStreamCount,
		},
		writer:    w,
		closeChan: make(chan struct{}, 1),
	}
	if len(qlogFile) > 0 {
		qc.config.Tracer = qlog.NewTracer(func(_ logging.Perspective, connID []byte) io.WriteCloser {
			f, err := os.Create(qlogFile)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Creating qlog file %s.\n", qlogFile)
			return util.NewBufferedWriteCloser(bufio.NewWriter(f), f)
		})
	}
	return qc
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
	var fbStream quic.SendStream
	if c.dgram {
		fbSender = func(fb []byte) error {
			return c.session.SendMessage(fb)
		}
	} else {
		fbSender = func(fb []byte) error {
			if fbStream == nil {
				var err error
				fbStream, err = c.session.OpenUniStreamSync(context.Background())
				if err != nil {
					return err
				}
			}
			length := uint32(len(fb))
			err := binary.Write(fbStream, binary.BigEndian, length)
			if err != nil {
				return err
			}
			_, err = fbStream.Write(fb)
			return err
		}
	}
	go func() {
		for {
			select {
			case fb := <-fbw:
				err := fbSender(fb)
				if err != nil {
					log.Println(err)
				}
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
		if len(bs) <= 2 {
			// ack provoking packet
			continue
		}
		_, err = io.Copy(c.writer, bytes.NewReader(bs))
		if err != nil && err != io.EOF {
			return err
		}
	}
}
