package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"log"
	"math/big"

	"github.com/mengelbart/cgo-streamer/gstsrc"

	"github.com/golang/protobuf/proto"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/quictrace"
	"github.com/lucas-clemente/quic-go/quictrace/pb"
)

const addr = "localhost:4242"

var tracer quictrace.Tracer

func init() {
	tracer = quictrace.NewTracer()
}

func main() {
	err := serve()
	if err != nil {
		panic(err)
	}
}

func serve() error {
	tlsConfig, err := generateTLSConfig()
	if err != nil {
		return err
	}
	max := uint64(1 << 60)
	listener, err := quic.ListenAddr(
		addr,
		tlsConfig,
		&quic.Config{
			QuicTracer:                            tracer,
			MaxReceiveStreamFlowControlWindow:     max,
			MaxReceiveConnectionFlowControlWindow: max,
			MaxIncomingStreams:                    int64(max),
			MaxIncomingUniStreams:                 int64(max),
		},
	)
	if err != nil {
		return err
	}
	sess, err := listener.Accept(context.Background())
	if err != nil {
		return err
	}

	//stream, err := sess.AcceptStream(context.Background())
	//if err != nil {
	//	return err
	//}

	log.Println("accepted stream, creating pipeline")

	gstsrc.CreatePipeline(&SingleStreamWriter{
		session: sess,
	})

	select {}
}

type OneStreamWriter struct {
	session quic.Session
	stream  quic.SendStream
}

func (o OneStreamWriter) Write(b []byte) (int, error) {
	if o.stream == nil {
		stream, err := o.session.OpenUniStream()
		if err != nil {
			return 0, err
		}
		o.stream = stream
	}
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, len(b))
	if err != nil {
		return 0, err
	}
	buf.Write(b)
	return o.stream.Write(buf.Bytes())
}

type SingleStreamWriter struct {
	session quic.Session
}

var numStreams = 0

func (s *SingleStreamWriter) Write(b []byte) (int, error) {
	stream, err := s.session.OpenStreamSync(context.Background())
	numStreams++
	log.Printf("opened %v streams", numStreams)
	if err != nil {
		return 0, err
	}
	defer func() {
		err := stream.Close()
		if err != nil {
			log.Printf("could not close stream: %v", err)
		}
		log.Printf("successfully closed stream")
	}()
	n, err := stream.Write(b)
	if err != nil {
		return 0, err
	}
	traces := tracer.GetAllTraces()
	if len(traces) != 1 {
		return 0, errors.New("expected excatly 1 trace")
	}
	for _, trace := range traces {
		tracePB := &pb.Trace{}
		err := proto.Unmarshal(trace, tracePB)
		if err != nil {
			return 0, err
		}
		for _, e := range tracePB.Events {
			log.Println(e.TransportState)
		}
	}

	return n, nil
}

func generateTLSConfig() (*tls.Config, error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-echo-example"},
	}, nil
}
