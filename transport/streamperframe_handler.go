package transport

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"

	"github.com/lucas-clemente/quic-go"
)

type StreamPerFrameHandler struct {
	src SrcFactory
}

func NewStreamPerFrameHandler(src SrcFactory) *StreamPerFrameHandler {
	return &StreamPerFrameHandler{
		src: src,
	}
}

func (m *StreamPerFrameHandler) handle(sess quic.Session) error {
	errChan := make(chan error, 1)
	session := &StreamPerFrameSession{
		session:  sess,
		err:      errChan,
		feedback: make(chan []byte, 1024),
	}
	go func() {
		err := session.AcceptFeedback()
		errChan <- err
	}()

	cancel := m.src.MakeSrc(session, session.feedback)
	defer cancel()

	var err error
	select {
	case err = <-errChan:
	}
	log.Println("closing streamperframe session")
	if err != nil {
		log.Println(err)
		return session.session.CloseWithError(1, err.Error())
	}
	return session.session.CloseWithError(0, "")
}

type StreamPerFrameSession struct {
	session  quic.Session
	err      chan error
	feedback chan []byte
}

func (m *StreamPerFrameSession) Close() error {
	m.err <- errors.New("eos")
	return nil
}

func (m *StreamPerFrameSession) AcceptFeedback() error {
	fbStream, err := m.session.AcceptUniStream(context.Background())
	if err != nil {
		return err
	}
	log.Println("accepted feedback stream")
	for {
		var size int32
		err := binary.Read(fbStream, binary.BigEndian, &size)
		if err != nil {
			log.Println(err)
			continue
		}
		fb := make([]byte, size)
		n, err := fbStream.Read(fb)
		if err != nil {
			log.Println(err)
			continue
		}
		if n != int(size) {
			log.Printf("got announcement of size %v feedback, but read only %v bytes", size, n)
		}
		m.feedback <- fb
	}
}

func (m *StreamPerFrameSession) Write(b []byte) (int, error) {
	stream, err := m.session.OpenStreamSync(context.Background())
	if err != nil {
		log.Println("could not open stream, closing session")
		m.err <- err
		return 0, err
	}
	defer func() {
		if stream != nil {
			err := stream.Close()
			if err != nil {
				log.Printf("could not Close stream: %v", err)
			}
		}
	}()

	n, err := io.Copy(stream, bytes.NewBuffer(b))
	if err != nil {
		if sErr, ok := err.(quic.StreamError); ok && sErr.Canceled() {
			log.Println("stream cancelled, closing session")
			m.err <- err
		}
		if nErr, ok := err.(net.Error); ok && nErr.Timeout() {
			log.Println("stream timeout, closing session")
			m.err <- err
		}
		return 0, err
	}

	return int(n), nil
}
