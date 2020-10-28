package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

var Scream bool
var Debug bool
var Handler string
var Addr string

func init() {
	rootCmd.PersistentFlags().BoolVarP(&Scream, "scream", "s", false, "Use scream congestion control")
	rootCmd.PersistentFlags().BoolVarP(&Debug, "verbose", "v", false, "Log debug output")
	rootCmd.PersistentFlags().StringVar(&Handler, "handler", "datagram", "Handler to use. Options are: udp, datagram, streamperframe")
	rootCmd.PersistentFlags().StringVarP(&Addr, "address", "a", "localhost:4242", "Address to bind to")
}

var rootCmd = &cobra.Command{
	Use: "qst",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
