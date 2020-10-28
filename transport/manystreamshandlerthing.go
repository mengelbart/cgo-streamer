package transport

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log"
	"net"

	"github.com/lucas-clemente/quic-go"
)

type SrcFactory interface {
	MakeSrc(writer io.WriteCloser, feedback <-chan []byte) func()
}

type ManyStreamsHandlerThing struct {
	Close    chan struct{}
	src      SrcFactory
	feedback chan []byte
}

func NewManyStreamsHandlerThing(src SrcFactory) *ManyStreamsHandlerThing {
	return &ManyStreamsHandlerThing{
		Close:    make(chan struct{}, 1),
		src:      src,
		feedback: make(chan []byte, 1024),
	}
}

func (m *ManyStreamsHandlerThing) handle(sess quic.Session) error {
	errChan := make(chan error, 1)
	handler := &ManyStreamWriterThing{
		session:  sess,
		err:      errChan,
		feedback: m.feedback,
	}
	go func() {
		err := handler.AcceptFeedback()
		errChan <- err
	}()
	cancel := m.src.MakeSrc(handler, m.feedback)
	defer cancel()
	select {
	case <-m.Close:
		return nil
	case err := <-errChan:
		return err
	}
}

type ManyStreamWriterThing struct {
	session  quic.Session
	err      chan error
	feedback chan []byte
}

func (m *ManyStreamWriterThing) Close() error {
	panic("implement me")
}

func (m *ManyStreamWriterThing) AcceptFeedback() error {
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

func (m *ManyStreamWriterThing) Write(b []byte) (int, error) {
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

	//traces := tracer.GetAllTraces()
	//if len(traces) != 1 {
	//	return 0, errors.New("expected excatly 1 trace")
	//}
	//for _, trace := range traces {
	//	tracePB := &pb.Trace{}
	//	err := proto.Unmarshal(trace, tracePB)
	//	if err != nil {
	//		return 0, err
	//	}
	//	for _, e := range tracePB.Events {
	//		log.Println(e.TransportState)
	//	}
	//}

	return int(n), nil
}
