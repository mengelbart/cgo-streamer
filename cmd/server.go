package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"sync"

	"github.com/lucas-clemente/quic-go/quictrace"

	"github.com/lucas-clemente/quic-go"
	"github.com/mengelbart/cgo-streamer/gst"
	"github.com/pion/rtp"

	"github.com/spf13/cobra"
)

var tracer quictrace.Tracer

func init() {
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use: "serve",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serve()
	},
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

	gst.StartMainLoop()
	gst.CreateSrcPipeline(&SingleStreamWriter{
		session: sess,
		//init:    sync.Once{},
	})

	select {}
}

type OneStreamWriter struct {
	session quic.Session
	stream  quic.SendStream
	init    sync.Once
}

func (o *OneStreamWriter) Write(b []byte) (int, error) {
	o.init.Do(func() {
		stream, err := o.session.OpenStreamSync(context.Background())
		if err != nil {
			panic(err)
		}
		o.stream = stream
	})
	log.Printf("writing %v bytes to pipeline", len(b))
	return o.stream.Write(b)
}

type SingleStreamWriter struct {
	session quic.Session
}

func (s *SingleStreamWriter) Write(b []byte) (int, error) {
	stream, err := s.session.OpenStreamSync(context.Background())
	if err != nil {
		return 0, err
	}
	defer func() {
		err := stream.Close()
		if err != nil {
			log.Printf("could not close stream: %v", err)
		}
	}()
	p := &rtp.Packet{}
	err = p.Unmarshal(b)
	if err != nil {
		panic(err)
	}
	fmt.Println(p)
	n, err := io.Copy(stream, bytes.NewBuffer(b))
	if err != nil {
		panic(err)
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
