package transport

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"log"
	"net"

	"github.com/lucas-clemente/quic-go"
)

type SrcFactory interface {
	MakeSrc(writer io.Writer) func()
	FeedbackChan() chan []byte
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
		feedback: src.FeedbackChan(),
	}
}

func (m *ManyStreamsHandlerThing) handle(sess quic.Session) error {
	errChan := make(chan error, 1)
	cancel := m.src.MakeSrc(&ManyStreamWriterThing{
		session:  sess,
		err:      errChan,
		feedback: m.feedback,
	})
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

func (m *ManyStreamWriterThing) AcceptFeedback() error {
	fbStream, err := m.session.AcceptUniStream(context.Background())
	if err != nil {
		return err
	}

	for {
		fb, err := ioutil.ReadAll(fbStream)
		if err != nil {
			log.Println(err)
			continue
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
