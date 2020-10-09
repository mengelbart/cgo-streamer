package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/mengelbart/cgo-streamer/gst"
	"github.com/mengelbart/cgo-streamer/packet"
	"github.com/mengelbart/cgo-streamer/transport"

	"github.com/lucas-clemente/quic-go/quictrace"
	"github.com/spf13/cobra"
)

var tracer quictrace.Tracer

var VideoSrc string
var LogRTP bool
var Handler string
var Addr string

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVar(&VideoSrc, "video-src", "videotestsrc", "Video file")
	serveCmd.Flags().StringVar(&Handler, "handler", "datagram", "Handler to use. Options are: datagram, streamperframe")
	serveCmd.Flags().BoolVar(&LogRTP, "logrtp", false, "Log RTP packets to stdout")
	serveCmd.Flags().StringVarP(&Addr, "address", "a", "localhost:4242", "Address to bind to")
}

var serveCmd = &cobra.Command{
	Use: "serve",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serve()
	},
}

func serve() error {
	if !Debug {
		log.SetOutput(ioutil.Discard)
	}
	src := &Src{
		videoSrc: VideoSrc,
		scream:   Scream,
		feedback: make(chan []byte, 1024),
	}
	if VideoSrc != "videotestsrc" {
		switch filepath.Ext(VideoSrc) {
		case ".mkv":
			src.videoSrc = fmt.Sprintf("filesrc location=%v ! matroskademux ! decodebin ! videoconvert", VideoSrc)
		case ".webm":
			src.videoSrc = fmt.Sprintf("filesrc location=%v ! matroskademux ! vp8dec ! videoconvert", VideoSrc)
		case ".mp4":
			src.videoSrc = fmt.Sprintf("filesrc location=%v ! decodebin ! videoconvert", VideoSrc)
		}
	}
	if LogRTP {
		src.logRTP()
	}

	var options []func(*transport.Server)
	if Handler == "streamperframe" {
		options = append(options, transport.SetSessionHandler(transport.NewManyStreamsHandlerThing(src)))
	} else {
		options = append(options, transport.SetSessionHandler(transport.NewDatagramHandler(src)))
		options = append(options, transport.SetDatagramEnabled(true))
	}

	s, err := transport.NewServer(
		Addr,
		nil,
		options...,
	)
	if err != nil {
		return err
	}
	return s.Run()
}

type Src struct {
	scream   bool
	videoSrc string
	writers  []io.Writer
	feedback chan []byte
}

func (s *Src) logRTP() {
	s.writers = append(s.writers, &packet.Logger{})
}

func (s *Src) MakeSrc(w io.Writer) func() {
	if s.scream {
		return s.MakeScreamSrc(w)
	}
	return s.MakeSimpleSrc(w)
}

func (s *Src) MakeSimpleSrc(w io.Writer) func() {

	mw := io.MultiWriter(append(s.writers, w)...)
	p := gst.NewSrcPipeline(mw, s.videoSrc)

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
	mw := io.MultiWriter(append(s.writers, w)...)
	ssrc := uint(1)
	cc := packet.NewScreamWriter(ssrc, mw, s.FeedbackChan())

	p := gst.NewSrcPipeline(cc, s.videoSrc)
	p.SetSSRC(ssrc)
	p.Start()
	go cc.Run()
	go cc.RunBitrate(make(chan struct{}, 1), p.SetBitRate)

	return func() {
		p.Stop()
		p.Destroy()
	}
}
