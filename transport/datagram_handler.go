package transport

import (
	"errors"
	"log"

	"github.com/lucas-clemente/quic-go"
)

type DatagramHandler struct {
	src SrcFactory
}

func NewDatagramHandler(src SrcFactory) *DatagramHandler {
	return &DatagramHandler{
		src: src,
	}
}

func (d *DatagramHandler) handle(session quic.Session) error {

	ds := &DatagramSession{
		sess:        session,
		feedback:    make(chan []byte, 1024),
		closeChan:   make(chan error, 1),
		feedbackErr: make(chan error, 1),
	}

	go ds.AcceptFeedback(ds.feedback)

	cancel := d.src.MakeSrc(ds, ds.feedback)
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

// TODO: Close properly, in case ReceiveMessage doesn't error out on session.Close?
func (d *DatagramSession) AcceptFeedback(fbChan chan<- []byte) {
	for {
		msg, err := d.sess.ReceiveMessage()
		if err != nil {
			d.feedbackErr <- err
		}
		fbChan <- msg
	}
}

func (d *DatagramSession) Write(b []byte) (int, error) {
	err := d.sess.SendMessage(b)
	return len(b), err
}
