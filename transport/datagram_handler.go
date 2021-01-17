package transport

import (
	"errors"
	"log"

	"github.com/lucas-clemente/quic-go"
)

type DatagramHandler struct {
	src    SrcFactory
	tracer *QUICTracer
}

func NewDatagramHandler(src SrcFactory, tracer *QUICTracer) *DatagramHandler {
	return &DatagramHandler{
		src:    src,
		tracer: tracer,
	}
}

func (d *DatagramHandler) handle(session quic.Session) error {

	ds := &DatagramSession{
		sess:        session,
		feedback:    make(chan []byte, 1024),
		closeChan:   make(chan error, 1),
		feedbackErr: make(chan error, 1),
	}

	go ds.AcceptFeedback()

	cancel := d.src.MakeSrc(ds, ds.feedback, d.tracer.getACKChan())
	defer cancel()

	var err error
	select {
	case err = <-ds.closeChan:
	case err = <-ds.feedbackErr:
	}
	log.Println("closing dgram session")
	if err != nil {
		log.Println(err)
		return ds.sess.CloseWithError(1, err.Error())
	}
	return ds.sess.CloseWithError(0, "")
}

func (d *DatagramSession) Close() error {
	d.closeChan <- errors.New("eos")
	return nil
}

type DatagramSession struct {
	sess        quic.Session
	feedback    chan []byte
	closeChan   chan error
	feedbackErr chan error
}

func (d *DatagramSession) AcceptFeedback() {
	for {
		msg, err := d.sess.ReceiveMessage()
		if err != nil {
			d.feedbackErr <- err
		}
		d.feedback <- msg
	}
}

func (d *DatagramSession) Write(b []byte) (int, error) {
	err := d.sess.SendMessage(b)
	return len(b), err
}
