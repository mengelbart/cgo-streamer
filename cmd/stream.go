package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"

	"github.com/mengelbart/cgo-streamer/transport"

	"github.com/mengelbart/cgo-streamer/gst"

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

func run() error {
	if !Debug {
		log.SetOutput(ioutil.Discard)
	}
	if VideoSink != "autovideosink" {
		VideoSink = fmt.Sprintf(" matroskamux ! filesink location=%v", VideoSink)
	} else {
		VideoSink = "videoconvert ! autovideosink"
	}
	gst.StartMainLoop()
	pipeline := gst.CreateSinkPipeline(VideoSink)
	destroyed := make(chan struct{}, 1)
	gst.HandleSinkEOS(func() {
		pipeline.Destroy()
		destroyed <- struct{}{}
	})
	pipeline.Start()
	var closeChans []chan<- struct{}

	var client FeedbackRunner
	if Scream {
		screamWriter := transport.NewScreamReadWriter(pipeline)
		closeChans = append(closeChans, screamWriter.CloseChan)
		client = newClient(Handler, Addr, screamWriter)
		sender, c, err := client.RunFeedbackSender()
		if err != nil {
			return err
		}
		closeChans = append(closeChans, c)
		go screamWriter.Run(sender)
	} else {
		client = newClient(Handler, Addr, pipeline)
	}
	closeChans = append(closeChans, client.CloseChan())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	done := make(chan struct{}, 1)
	var err error
	go func() {
		err = client.Run()
		log.Println("client run done")
		close(done)
		for _, c := range closeChans {
			close(c)
		}
	}()

	select {
	case sig := <-signals:
		log.Println(sig)
	case <-done:
	}

	log.Println("stopping pipeline")
	pipeline.Stop()
	<-destroyed

	log.Println("exiting")
	return err
}

type FeedbackRunner interface {
	Runner
	RunFeedbackSender() (io.Writer, chan<- struct{}, error)
	CloseChan() chan struct{}
}

func newClient(handler string, addr string, w io.Writer) FeedbackRunner {
	switch handler {
	case "udp":
		return transport.NewUDPClient(addr, w)
	case "streamperframe":
		return transport.NewQUICClient(addr, w, false)
	case "datagram":
		fallthrough
	default:
		return transport.NewQUICClient(addr, w, true)
	}
}
