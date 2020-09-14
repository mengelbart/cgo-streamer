package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"log"
	"math/big"

	"github.com/mengelbart/cgo-streamer/gstsrc"

	"github.com/lucas-clemente/quic-go"
)

const addr = "localhost:4242"

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
	listener, err := quic.ListenAddr(addr, tlsConfig, nil)
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

type SingleStreamWriter struct {
	session quic.Session
}

func (s *SingleStreamWriter) Write(b []byte) (n int, err error) {
	stream, err := s.session.OpenStreamSync(context.Background())
	if err != nil {
		return 0, err
	}
	defer stream.Close()
	return stream.Write(b)
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
