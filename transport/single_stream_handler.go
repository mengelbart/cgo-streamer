package transport

import (
	"context"
	"log"
	"sync"

	"github.com/lucas-clemente/quic-go"
)

type OneStreamWriter struct {
	session quic.Session
	stream  quic.SendStream
	init    sync.Once
}

func (o *OneStreamWriter) Write(b []byte) (int, error) {
	var err error
	o.init.Do(func() {
		stream, e := o.session.OpenStreamSync(context.Background())
		if e != nil {
			err = e
		}
		o.stream = stream
	})
	if err != nil {
		return 0, err
	}
	log.Printf("writing %v bytes to pipeline", len(b))
	return o.stream.Write(b)
}
