package cmd

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/mengelbart/cgo-streamer/transport"

	"github.com/mengelbart/cgo-streamer/benchmark"

	"github.com/spf13/cobra"
)

var (
	Version    string
	Commit     string
	Timestamp  string
	Upload     bool
	InputFiles []string
)

func init() {
	rootCmd.AddCommand(benchmarkCmd)
	benchmarkCmd.Flags().StringSliceVarP(&InputFiles, "input-files", "f", []string{}, "List of video files to include in test runs. Use \"comma,separated,list\" or add flag multiple times")
	benchmarkCmd.Flags().BoolVarP(&Upload, "upload", "u", false, "Upload results to google cloud. Requires GOOGLE_APPLICATION_CREDENTIALS to be set to a valid value (see https://developers.google.com/accounts/docs/application-default-credentials for more information)")
}

var benchmarkCmd = &cobra.Command{
	Use: "bench",
	Long: `bench runs a number of pre-configured experiments.
This command does not respect any of the global flags because
it uses pre-configured values for all executed experiments.

The command requires two network interfaces in two different
Linux network namespaces which can be configured with the 
"vnetns.sh" script.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBenchmark()
	},
}

var bandwidths = []benchmark.Bitrate{
	0,
	1 * benchmark.MBitPerSecond,
	2 * benchmark.MBitPerSecond,
	3 * benchmark.MBitPerSecond,
	4 * benchmark.MBitPerSecond,
}
var congestionControllers = []string{
	"none",
	"scream",
}
var handlers = []string{
	"udp",
	"streamperframe",
	"datagram",
}
var feedbackFrequencies = []time.Duration{
	1 * time.Millisecond,
	10 * time.Millisecond,
	20 * time.Millisecond,
	100 * time.Millisecond,
}
var feedbackAlgorithms = []transport.FeedbackAlgorithm{
	transport.StaticDelay,
	transport.ACKTimestamp,
}

func runBenchmark() error {
	log.Println(version())
	evaluator := benchmark.Evaluator{
		InputFiles:            InputFiles,
		Bandwidths:            bandwidths,
		CongestionControllers: congestionControllers,
		Handlers:              handlers,
		FeedbackFrequencies:   feedbackFrequencies,
		RequestKeyFrames:      []bool{false}, //, true},
		Iperf:                 []bool{false, true},
		FeedbackAlgorithms:    feedbackAlgorithms,
	}
	return evaluator.RunAll(
		dataDir,
		Version,
		Commit,
		Timestamp,
		addr,
		port,
		Upload,
	)
}

const (
	dataDir = "data"
	addr    = "192.168.1.11"
	port    = "4242"
)

func version() (string, error) {
	if len(Version) == 0 {
		return "", errors.New("empty version string")
	}
	if len(Commit) == 0 {
		return "", errors.New("emtpy commit string")
	}
	if len(Timestamp) == 0 {
		return "", errors.New("empty timestamp string")
	}

	i, err := strconv.ParseInt(Timestamp, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid timestamp format: %v", Timestamp)
	}
	buildTime := time.Unix(i, 0)

	return fmt.Sprintf("Version: %v, Build Commit: %v, Timestamp: %v", Version, Commit, buildTime), nil
}
