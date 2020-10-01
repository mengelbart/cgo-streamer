package quic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/lucas-clemente/quic-go"
	"github.com/mengelbart/cgo-streamer/gst"
	"github.com/pion/rtp"
)

type ManyStreamsHandlerThing struct {
	close chan struct{}
}

func (m *ManyStreamsHandlerThing) handle(sess quic.Session) error {
	errChan := make(chan error, 1)
	gst.CreateSrcPipeline(&ManyStreamWriterThing{
		session: sess,
		err:     errChan,
	})
	select {
	case <-m.close:
		return nil
	case err := <-errChan:
		return err
	}
}

type ManyStreamWriterThing struct {
	session quic.Session
	err     chan error
}

func (m *ManyStreamWriterThing) Write(b []byte) (int, error) {
	stream, err := m.session.OpenStreamSync(context.Background())
	if err != nil {
		log.Println("could not open stream, closing session")
		m.err <- err
	}
	defer func() {
		if stream != nil {
			err := stream.Close()
			if err != nil {
				log.Printf("could not close stream: %v", err)
			}
		}
	}()
	p := &rtp.Packet{}
	err = p.Unmarshal(b)
	if err != nil {
		return 0, err
	}
	fmt.Println(p)

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
