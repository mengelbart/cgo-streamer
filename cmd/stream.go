package cmd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"io"
	"io/ioutil"
	"log"
	"time"

	"github.com/mengelbart/scream-go"

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
	pipeline := gst.CreateSinkPipeline()

	rx := scream.NewRx()

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
		//fmt.Println(packet)
		packetChan <- packet

		_, err = io.Copy(pipeline, bytes.NewReader(bs))
		if err != nil && err != io.EOF {
			panic(err)
			return err
		}
	}
}
