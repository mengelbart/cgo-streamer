package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/mengelbart/cgo-streamer/transport"

	"github.com/spf13/cobra"
)

var Scream bool
var Debug bool
var Handler string
var Addr string
var QLOGFile string
var FeedbackAlgorithm string

func init() {
	log.SetFlags(log.Lmicroseconds)
	rootCmd.PersistentFlags().BoolVarP(&Scream, "scream", "s", false, "Use scream congestion control")
	rootCmd.PersistentFlags().BoolVarP(&Debug, "verbose", "v", false, "Log debug output")
	rootCmd.PersistentFlags().StringVar(&Handler, "handler", "datagram", "Handler to use. Options are: udp, datagram, streamperframe")
	rootCmd.PersistentFlags().StringVarP(&Addr, "address", "a", "localhost:4242", "Address to bind to")
	rootCmd.PersistentFlags().StringVarP(&QLOGFile, "qlog", "q", "", "Enable QLOG and write to given filename")
	rootCmd.PersistentFlags().StringVar(
		&FeedbackAlgorithm,
		"feedback-algorithm",
		"receive",
		fmt.Sprintf("Choose an algorithm to generate SCReAM feedback: \n"+
			"%v: Send normal feedback from receiver to sender (default)\n"+
			"%v: Infer feedback using static interval\n"+
			"%v: Infer feedback QUIC ACK timestamp\n"+
			"%v: Infer feedback from sent timestamp and current smoothed RTT",
			transport.Receive, transport.StaticDelay, transport.ACKTimestamp, transport.RTTArrival))
}

var rootCmd = &cobra.Command{
	Use: "qrt",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
