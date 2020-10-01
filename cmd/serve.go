package cmd

import (
	"io"

	"github.com/mengelbart/cgo-streamer/gst"
	"github.com/mengelbart/cgo-streamer/packet"
	"github.com/mengelbart/cgo-streamer/transport"

	"github.com/lucas-clemente/quic-go/quictrace"
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
	src := &Src{}
	src.logRTP()

	s, err := transport.NewServer(
		"localhost:4242",
		nil,
		transport.SetSessionHandler(transport.NewManyStreamsHandlerThing(src)),
	)
	if err != nil {
		return err
	}
	return s.Run()
}

type Src struct {
	writers  []io.Writer
	feedback chan []byte
}

func (s *Src) logRTP() {
	s.writers = append(s.writers, &packet.Logger{})
}

func (s *Src) MakeSrc(w io.Writer) func() {

	mw := io.MultiWriter(append(s.writers, w)...)
	p := gst.NewSrcPipeline(mw)

	p.Start()
	go func() {
		for {
			// ignore feedback chan to avoid getting stuck when channel is full
			<-s.feedback
		}
	}()

	return func() {
		p.Stop()
		p.Destroy()
	}
}

func (s *Src) FeedbackChan() chan []byte {
	return s.feedback
}

func (s *Src) MakeScreamSrc(w io.Writer) func() {
	ssrc := uint(1)
	cc := packet.NewScreamWriter(ssrc, w, s.FeedbackChan())

	p := gst.NewSrcPipeline(cc)
	p.SetSSRC(ssrc)
	p.Start()
	go cc.Run()

	return func() {
		p.Stop()
		p.Destroy()
	}
}
