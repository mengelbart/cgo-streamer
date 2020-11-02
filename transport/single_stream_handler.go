package transport

import (
	"context"
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
	return o.stream.Write(b)
}
