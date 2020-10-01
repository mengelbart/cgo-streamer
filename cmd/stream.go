package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"

	"github.com/mengelbart/cgo-streamer/gst"

	"github.com/lucas-clemente/quic-go"
	"github.com/pion/rtp"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(streamCmd)
}

var streamCmd = &cobra.Command{
	Use: "stream",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run()
	},
}

const addr = "localhost:4242"

func run() error {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
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
	pipeline := gst.CreateSinkPipeline()

	for {

		stream, err := session.AcceptStream(context.Background())
		if err != nil {
			return err
		}

		bs, err := ioutil.ReadAll(stream)
		packet := &rtp.Packet{}
		err = packet.Unmarshal(bs)
		if err != nil {
			return err
		}
		fmt.Println(packet)

		_, err = io.Copy(pipeline, bytes.NewReader(bs))
		if err != nil && err != io.EOF {
			return err
		}
	}
}
