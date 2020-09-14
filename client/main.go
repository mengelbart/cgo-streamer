package main

import (
	"context"
	"crypto/tls"
	"io"
	"log"

	"github.com/lucas-clemente/quic-go"
	"github.com/mengelbart/cgo-streamer/gstsink"
)

const addr = "localhost:4242"

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}

func run() error {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}
	session, err := quic.DialAddr(addr, tlsConf, nil)
	if err != nil {
		return err
	}

	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}

	_, err = stream.Write([]byte("hello"))
	if err != nil {
		return err
	}

	log.Println("opened stream, creating pipeline")

	pipeline := gstsink.CreatePipeline()

	buf := make([]byte, 10240)
	for {

		stream, err := session.AcceptStream(context.Background())
		if err != nil {
			return err
		}

		n, err := stream.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		//log.Printf("%v\n", string(buf[:64]))
		_, err = pipeline.Write(buf[:n])
		if err != nil {
			return err
		}
	}
}
