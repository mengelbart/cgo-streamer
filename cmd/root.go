package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

var Scream bool
var Debug bool

func init() {
	rootCmd.PersistentFlags().BoolVarP(&Scream, "scream", "s", false, "Use scream congestion control")
	rootCmd.PersistentFlags().BoolVarP(&Debug, "verbose", "v", false, "Log debug output")
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
