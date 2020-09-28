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
	rootCmd.AddCommand(clientCmd)
}

var clientCmd = &cobra.Command{
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

	//stream, err = session.AcceptStream(context.Background())
	//if err != nil {
	//	return err
	//}
	//buf := make([]byte, 1500)
	//var n int
	//for err != io.EOF {
	//	n, err = stream.Read(buf)
	//	if err != nil && err != io.EOF {
	//		return err
	//	}
	//	log.Printf("read %v bytes from stream", n)
	//	_, err = pipeline.Write(buf[:n])
	//	if err != nil && err != io.EOF {
	//		return err
	//	}
	//}
	//return nil

	//buf := make([]byte, 10240)
	for {

		stream, err := session.AcceptStream(context.Background())
		if err != nil {
			return err
		}

		bs, err := ioutil.ReadAll(stream)
		packet := &rtp.Packet{}
		err = packet.Unmarshal(bs)
		if err != nil {
			panic(err)
			//return 0, err
		}
		fmt.Println(packet)

		_, err = io.Copy(pipeline, bytes.NewReader(bs))
		//n, err := stream.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		//log.Printf("%v\n", string(buf[:64]))
		//_, err = pipeline.Write(buf[:n])
		//if err != nil {
		//	return err
		//}
	}
}
