package cmd

import (
	"github.com/mengelbart/cgo-streamer/quic"

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
	s, err := quic.NewServer(
		"localhost:4242",
		nil,
		quic.SetSessionHandler(&quic.ManyStreamsHandlerThing{}),
	)
	if err != nil {
		return err
	}
	return s.Run()
}
