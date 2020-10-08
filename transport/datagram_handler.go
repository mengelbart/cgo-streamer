package transport

import (
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

type DatagramSession struct {
	sess        quic.Session
	feedback    chan []byte
	closeChan   chan error
	feedbackErr chan error
}

func (d *DatagramHandler) handle(session quic.Session) error {

	ds := &DatagramSession{
		sess:        session,
		feedback:    d.src.FeedbackChan(),
		closeChan:   make(chan error, 1),
		feedbackErr: make(chan error, 1),
	}

	go ds.AcceptFeedback()

	cancel := d.src.MakeSrc(ds)
	defer cancel()

	var err error
	select {
	case err = <-ds.closeChan:
	case err = <-ds.feedbackErr:
	}
	return ds.sess.CloseWithError(0, err.Error())
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
	if err != nil {
		d.closeChan <- err
		return 0, err
	}
	return len(b), nil
}
