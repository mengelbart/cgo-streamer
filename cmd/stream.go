package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/mengelbart/cgo-streamer/transport"

	"github.com/mengelbart/cgo-streamer/gst"

	"github.com/spf13/cobra"
)

var VideoSink string
var FeedbackFreq int
var SendImmediateFeedback bool
var RTCPLogFile string

func init() {
	rootCmd.AddCommand(streamCmd)
	streamCmd.Flags().StringVar(&VideoSink, "video-sink", "autovideosink", "File to save video")
	streamCmd.Flags().IntVarP(&FeedbackFreq, "feedback-frequency", "f", 500, "Frequency in which scream feedback is sent in ms")
	streamCmd.Flags().BoolVarP(&SendImmediateFeedback, "immediate-feedback", "i", false, "Send SCReAM Feedback immediately when a new RTP Packet was received.")
	streamCmd.Flags().StringVar(&RTCPLogFile, "rtcp-logger", "stdout", "Log file for rtcp statistics, 'stdout' prints to stdout, otherwise creates a new file")
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
		screamWriter := transport.NewScreamReadWriter(pipeline, time.Duration(FeedbackFreq)*time.Millisecond, SendImmediateFeedback)
		closeChans = append(closeChans, screamWriter.CloseChan)
		client = newClient(Handler, Addr, screamWriter, QLOGFile)
		sender, c, err := client.RunFeedbackSender()
		if err != nil {
			return err
		}
		closeChans = append(closeChans, c)
		writer, cancel, err := getRTCPStatWriter(sender, RTCPLogFile)
		if err != nil {
			return err
		}
		defer cancel()
		if FeedbackAlgorithm > 0 {
			go screamWriter.RunMinimalFeedback(writer)
		} else {
			go screamWriter.RunFullFeedback(writer)
		}
	} else {
		client = newClient(Handler, Addr, pipeline, QLOGFile)
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

type rtcpStatsWriter struct {
	l       *log.Logger
	t       time.Time
	counter chan int
	stop    chan struct{}
}

func (r *rtcpStatsWriter) run() {
	count := 0
	t := time.NewTicker(1 * time.Second)
	for {
		select {
		case c := <-r.counter:
			count += c
		case <-t.C:
			r.l.Printf("%v %v", time.Since(r.t).Milliseconds(), count)
			count = 0
		case <-r.stop:
			return
		}
	}
}

func (r rtcpStatsWriter) Write(p []byte) (int, error) {
	r.counter <- len(p)
	return len(p), nil
}

func (r *rtcpStatsWriter) Close() {
	close(r.stop)
}

func getRTCPStatWriter(w io.Writer, rtcpStatsFile string) (io.Writer, func(), error) {
	create, err := os.Create(rtcpStatsFile)
	if err != nil {
		return nil, nil, err
	}
	rtcpWriter := rtcpStatsWriter{
		l:       log.New(create, "", 0),
		t:       time.Now(),
		counter: make(chan int),
		stop:    make(chan struct{}),
	}
	go rtcpWriter.run()
	return io.MultiWriter(w, rtcpWriter), rtcpWriter.Close, nil
}

func newClient(handler string, addr string, w io.Writer, qlogFile string) FeedbackRunner {
	switch handler {
	case "udp":
		return transport.NewUDPClient(addr, w)
	case "streamperframe":
		return transport.NewQUICClient(addr, w, false, qlogFile)
	case "datagram":
		fallthrough
	default:
		return transport.NewQUICClient(addr, w, true, qlogFile)
	}
}
