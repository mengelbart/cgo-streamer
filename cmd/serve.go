package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/mengelbart/cgo-streamer/gst"
	"github.com/mengelbart/cgo-streamer/transport"

	"github.com/lucas-clemente/quic-go/quictrace"
	"github.com/spf13/cobra"
)

var tracer quictrace.Tracer

var VideoSrc string
var Bitrate int
var ScreamLogFile string

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVar(&VideoSrc, "video-src", "videotestsrc", "Video file")
	serveCmd.Flags().IntVarP(&Bitrate, "bitrate", "b", 2048, "initial encoder bitrate")
	serveCmd.Flags().StringVar(&ScreamLogFile, "scream-logger", "stdout", "Log file for scream statistics, 'stdout' prints to stdout, otherwise creates a new file")
}

var serveCmd = &cobra.Command{
	Use: "serve",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serve()
	},
}

type Runner interface {
	Run() error
}

func serve() error {
	if !Debug {
		log.SetOutput(ioutil.Discard)
	}
	src := &Src{
		videoSrc: VideoSrc,
		scream:   Scream,
		bitrate:  Bitrate,
	}
	if Scream {
		if ScreamLogFile != "stdout" {
			create, err := os.Create(ScreamLogFile)
			if err != nil {
				return err
			}
			src.ScreamLogWriter = create
		} else {
			src.ScreamLogWriter = os.Stdout
		}
	} else {
		src.ScreamLogWriter = ioutil.Discard
	}
	if VideoSrc != "videotestsrc" {
		src.videoSrc = fmt.Sprintf("filesrc location=%v ! queue ! decodebin ! videoconvert ", VideoSrc)
	}

	var runner Runner
	var options []func(*transport.QUICServer)
	if len(QLOGFile) > 0 {
		options = append(options, transport.EnableQLOG(QLOGFile))
	}
	switch Handler {
	case "udp":
		runner = transport.NewUDPServer(Addr, transport.SetPacketHandler(transport.NewUDPPacketHandler(src)))
	case "streamperframe":
		options = append(options, transport.SetSessionHandler(transport.NewStreamPerFrameHandler(src)))
		s, err := transport.NewQUICServer(Addr, nil, options...)
		if err != nil {
			return err
		}
		runner = s
	case "datagram":
		options = append(options, transport.SetSessionHandler(transport.NewDatagramHandler(src)))
		options = append(options, transport.SetDatagramEnabled(true))
		fallthrough
	default:
		s, err := transport.NewQUICServer(Addr, nil, options...)
		if err != nil {
			return err
		}
		runner = s
	}

	return runner.Run()
}

type Src struct {
	scream          bool
	ScreamLogWriter io.Writer
	videoSrc        string
	bitrate         int
}

func (s *Src) MakeSrc(w io.WriteCloser, fb <-chan []byte) func() {
	if s.scream {
		return s.MakeScreamSrc(w, fb)
	}
	return s.MakeSimpleSrc(w, fb)
}

func (s *Src) MakeSimpleSrc(w io.WriteCloser, fb <-chan []byte) func() {

	p := gst.NewSrcPipeline(w, s.videoSrc, s.bitrate)

	p.Start()
	go func() {
		for {
			// ignore feedback chan to avoid getting stuck when channel is full
			<-fb
		}
	}()

	return func() {
		p.Stop()
		p.Destroy()
	}
}

func (s *Src) MakeScreamSrc(w io.WriteCloser, fb <-chan []byte) func() {
	ssrc := uint(1)
	cc := transport.NewScreamWriter(ssrc, s.bitrate, w, fb, s.ScreamLogWriter)

	p := gst.NewSrcPipeline(cc, s.videoSrc, s.bitrate)
	p.SetSSRC(ssrc)
	cc.SetKeyFrameRequester(p.ForceKeyFrame)
	p.Start()
	go cc.Run()
	go cc.RunBitrate(p.SetBitRate)

	return func() {
		p.Stop()
		p.Destroy()
	}
}
