package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/mengelbart/cgo-streamer/transport"

	"github.com/mengelbart/scream-go"

	"github.com/mengelbart/cgo-streamer/gst"

	"github.com/lucas-clemente/quic-go"
	"github.com/pion/rtp"
	"github.com/spf13/cobra"
)

var VideoSink string

func init() {
	rootCmd.AddCommand(streamCmd)
	streamCmd.Flags().StringVar(&VideoSink, "video-sink", "autovideosink", "File to save video")
}

var streamCmd = &cobra.Command{
	Use: "stream",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

const addr = "localhost:4242"

func run() error {
	if !Debug {
		log.SetOutput(ioutil.Discard)
	}
	vSink := VideoSink
	if vSink != "autovideosink" {
		vSink = fmt.Sprintf(" queue ! x264enc ! mp4mux ! filesink location=%v", VideoSink)
	} else {
		vSink = "videoconvert ! autovideosink"
	}
	gst.StartMainLoop()
	pipeline := gst.CreateSinkPipeline(vSink)
	pipeline.Start()
	var closeChans []chan<- struct{}

	var client FeedbackRunner
	if Scream {
		screamWriter := transport.NewScreamReadWriter(pipeline)
		closeChans = append(closeChans, screamWriter.CloseChan)
		client = newClient(Handler, addr, screamWriter)
		sender, c := client.RunFeedbackSender()
		closeChans = append(closeChans, c)
		go screamWriter.Run(sender)
	} else {
		client = newClient(Handler, addr, pipeline)
	}
	closeChans = append(closeChans, client.CloseChan())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	var err error
	go func() {
		err = client.Run()
	}()

	sig := <-signals
	log.Println(sig)
	log.Println("stopping pipeline")
	pipeline.Stop()
	time.Sleep(3 * time.Second)
	pipeline.Destroy()
	for _, c := range closeChans {
		close(c)
	}

	log.Println("exiting")
	return err
}

type FeedbackRunner interface {
	Runner
	RunFeedbackSender() (io.Writer, chan<- struct{})
	CloseChan() chan struct{}
}

func newClient(handler string, addr string, w io.Writer) FeedbackRunner {
	switch handler {
	case "udp":
		return transport.NewUDPClient(addr, w)
	case "streamperframe":
		panic("streamperframe client not implemented")
	case "datagram":
		fallthrough
	default:
		return transport.NewQUICClient(addr, w)
	}
}

func runOld() error {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-realtime"},
	}
	max := uint64(1 << 60)
	session, err := quic.DialAddr(
		addr,
		tlsConf,
		&quic.Config{
			MaxIncomingStreams:                    int64(max),
			MaxIncomingUniStreams:                 int64(max),
			MaxReceiveStreamFlowControlWindow:     max,
			MaxReceiveConnectionFlowControlWindow: max,
		},
	)
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

	gst.StartMainLoop()
	pipeline := gst.CreateSinkPipeline("videotestsrc")

	rx := scream.NewRx(1)

	fbStream, err := session.OpenUniStream()
	if err != nil {
		return err
	}
	packetChan := make(chan *rtp.Packet, 1024)
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case p := <-packetChan:
				rx.Receive(
					uint(time.Now().UTC().Unix()),
					nil,
					int(p.SSRC),
					len(p.Raw),
					int(p.SequenceNumber),
					0,
				)
			case <-ticker.C:
			}
			if ok, feedback := rx.CreateStandardizedFeedback(
				uint(time.Now().UTC().Unix()),
				true,
			); ok {
				err := binary.Write(fbStream, binary.BigEndian, int32(len(feedback)))
				if err != nil {
					log.Println(err)
				}
				_, err = fbStream.Write(feedback)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}()

	for {

		stream, err := session.AcceptStream(context.Background())
		if err != nil {
			return err
		}

		bs, err := ioutil.ReadAll(stream)
		if err != nil {
			return err
		}
		packet := &rtp.Packet{}
		err = packet.Unmarshal(bs)
		if err != nil {
			return err
		}
		packetChan <- packet

		_, err = io.Copy(pipeline, bytes.NewReader(bs))
		if err != nil && err != io.EOF {
			panic(err)
			return err
		}
	}
}
