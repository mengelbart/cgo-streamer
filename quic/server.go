package quic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"log"
	"math/big"

	"github.com/lucas-clemente/quic-go/quictrace"

	"github.com/lucas-clemente/quic-go"
)

const maxControlWindowSize = uint64(1 << 60)
const maxStreamCount = int64(1 << 60)

type SessionHandler interface {
	handle(session quic.Session) error
}

type defaultSessionHandler string

func (d defaultSessionHandler) handle(session quic.Session) error {
	return session.CloseWithError(0, string(d))
}

type Server struct {
	SessionHandler
	addr       string
	tlsConfig  *tls.Config
	quicConfig *quic.Config
}

func NewServer(addr string, tlsc *tls.Config, options ...func(*Server)) (*Server, error) {
	s := &Server{
		addr:      addr,
		tlsConfig: tlsc,
		quicConfig: &quic.Config{
			//MaxReceiveStreamFlowControlWindow:     maxControlWindowSize,
			//MaxReceiveConnectionFlowControlWindow: maxControlWindowSize,
			MaxIncomingStreams:    maxStreamCount,
			MaxIncomingUniStreams: maxStreamCount,
		},
	}
	for _, option := range options {
		option(s)
	}
	if s.tlsConfig == nil {
		config, err := generateTLSConfig()
		if err != nil {
			return nil, err
		}
		s.tlsConfig = config
	}
	if s.SessionHandler == nil {
		s.SessionHandler = defaultSessionHandler("No handler defined on this server, closing")
	}
	return s, nil
}

func SetQuicTracer(t quictrace.Tracer) func(*Server) {
	return func(s *Server) {
		s.quicConfig.QuicTracer = t
	}
}

func SetSessionHandler(sh SessionHandler) func(*Server) {
	return func(s *Server) {
		s.SessionHandler = sh
	}
}

func (s *Server) Run() error {
	listener, err := quic.ListenAddr(
		s.addr,
		s.tlsConfig,
		s.quicConfig,
	)
	if err != nil {
		return err
	}
	return s.accept(listener)
}

func (s *Server) accept(listener quic.Listener) error {
	for {
		sess, err := listener.Accept(context.Background())
		if err != nil {
			return err
		}
		log.Printf("session accepted: %s", sess.RemoteAddr().String())
		go func() {
			var err error
			defer func() {
				if err != nil {
					log.Printf("closing session with error: %v\n", err)
					err = sess.CloseWithError(1, err.Error())
					log.Printf("error while closing session: %v\n", err)
					return
				}
				err := sess.CloseWithError(0, "bye")
				if err != nil {
					log.Printf("error while closing session: %v\n", err)
					return
				}
				log.Println("closed session")
			}()
			err = s.handle(sess)
		}()
	}
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
