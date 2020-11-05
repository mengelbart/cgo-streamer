package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var (
	Version   string
	Commit    string
	Timestamp string
)

func init() {
	rootCmd.AddCommand(benchmarkCmd)
}

var benchmarkCmd = &cobra.Command{
	Use: "bench",
	Run: func(cmd *cobra.Command, args []string) {
		benchmark()
	},
}

const (
	dataDir = "data"
	addr    = "192.168.1.11:4242"
)

type bitrate int64

const (
	bitPerSecond  bitrate = 1
	kBitPerSecond         = 1000 * bitPerSecond
)

type config struct {
	Filename          string  `json:"filename"`
	AbsFile           string  `json:"absoluteFilename"`
	BaseFile          string  `json:"baseFilename"`
	Bandwidth         bitrate `json:"bandwidth"`
	CongestionControl string  `json:"congestionControl"`
	Handler           string  `json:"handler"`

	Version string `json:"version"`
}

func (c config) String() string {
	return fmt.Sprintf("%v-%v-%v-%v", c.BaseFile, c.Bandwidth, c.CongestionControl, c.Handler)
}

func (c config) serveCmd() []string {
	cmd := []string{
		"serve",
		"-a",
		addr,
		"--video-src",
		c.AbsFile,
		"--handler",
		c.Handler,
	}

	if c.CongestionControl == "scream" {
		cmd = append(cmd, "-s")
	}
	return cmd
}

func (c config) clientCmd() []string {
	cmd := []string{
		"stream",
		"-a",
		addr,
		"--video-sink",
		fmt.Sprintf("streamed-%v", c.BaseFile),
		"--handler",
		c.Handler,
	}

	if c.CongestionControl == "scream" {
		cmd = append(cmd, "-s")
	}
	return cmd
}

var configs = []*config{
	{
		Filename:          "input/Sintel_480_snippet_yuv.mkv",
		Bandwidth:         1000 * kBitPerSecond,
		CongestionControl: "scream",
		Handler:           "datagram",
	},
	{
		Filename:          "input/Sintel_480_snippet_yuv.mkv",
		Bandwidth:         1000 * kBitPerSecond,
		CongestionControl: "scream",
		Handler:           "udp",
	},
}

func initConfigs(raw []*config) []*config {
	for _, c := range raw {
		abs, err := filepath.Abs(c.Filename)
		if err != nil {
			panic(err)
		}
		base := filepath.Base(c.Filename)
		if err != nil {
			panic(err)
		}
		c.AbsFile = abs
		c.BaseFile = base

		v, err := version()
		if err != nil {
			panic(err)
		}
		c.Version = v
	}
	return raw
}

func benchmark() {
	version, err := version()
	if err != nil {
		panic(err)
	}
	fmt.Println(version)

	bin, err := os.Executable()
	if err != nil {
		panic(err)
	}
	plotter, err := filepath.Abs("plot.py")
	if err != nil {
		panic(err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Printf("could not get hostname, using 'unknownhost', err: %v\n", err)
		hostname = "unknownhost"
	}

	expDir := dataDir + string(filepath.Separator) + Commit + string(filepath.Separator) + hostname
	err = os.MkdirAll(expDir, os.ModePerm)
	if err != nil {
		panic(err)
	}

	cs := initConfigs(configs)

	err = os.Chdir(expDir)
	if err != nil {
		panic(err)
	}
	for _, c := range cs {
		func() {
			err = os.Mkdir(c.String(), os.ModePerm)
			if err != nil {
				panic(err)
			}
			err = os.Chdir(c.String())
			if err != nil {
				panic(err)
			}
			defer func() {
				err = os.Chdir("..")
				if err != nil {
					panic(err)
				}
			}()

			file, err := json.MarshalIndent(c, "", "	")
			if err != nil {
				panic(err)
			}
			err = ioutil.WriteFile("config.json", file, 0644)
			if err != nil {
				panic(err)
			}

			if c.Bandwidth > 0 {
				for i := 1; i <= 2; i++ {
					tc := exec.Command("tc", "-n", fmt.Sprintf("ns%v", i), "qdisc", "add", "dev", fmt.Sprintf("veth%v", i), "root", "netem", "rate", fmt.Sprintf("%v", c.Bandwidth))
					tc.Stdout = os.Stdout
					tc.Stderr = os.Stderr
					err = tc.Run()
					if err != nil {
						fmt.Printf("tc add for ns%v returned error: %v\n", i, err)
					}
					defer func(i int) {
						tc := exec.Command("tc", "-n", fmt.Sprintf("ns%v", i), "qdisc", "delete", "dev", fmt.Sprintf("veth%v", i), "root")
						tc.Stdout = os.Stdout
						tc.Stderr = os.Stderr
						err = tc.Run()
						if err != nil {
							fmt.Printf("tc delete for ns%v returned error: %v\n", i, err)
						}
					}(i)
				}
			}

			serveLog := "serve.log"
			serveLogFile, err := os.Create(serveLog)
			if err != nil {
				fmt.Printf("could not touch serve log: %v", err)
				return
			}
			defer serveLogFile.Close()
			serve := exec.Command("ip", append([]string{"netns", "exec", "ns1", bin}, c.serveCmd()...)...)
			serve.Stdout = serveLogFile
			serve.Stderr = serveLogFile
			err = serve.Start()
			if err != nil {
				fmt.Printf("could not run server: %v\n", err)
				return
			}
			defer func() {
				if err := serve.Process.Kill(); err != nil {
					fmt.Printf("could not kill serve cmd: %v\n", err)
				}
			}()

			clientLog := "client.log"
			clientLogFile, err := os.Create(clientLog)
			if err != nil {
				fmt.Printf("could not touch client log: %v", err)
				return
			}
			stream := exec.Command("ip", append([]string{"netns", "exec", "ns2", bin}, c.clientCmd()...)...)
			stream.Stdout = clientLogFile
			stream.Stderr = clientLogFile
			err = stream.Run()
			if err != nil {
				fmt.Printf("could not run stream client: %v\n", err)
			}

			ffmpegLog := "ffmpeg.log"
			ffmpegLogFile, err := os.Create(ffmpegLog)
			if err != nil {
				fmt.Printf("could not touch ffmpeg log: %v", err)
				return
			}
			ffmpeg := exec.Command(
				"ffmpeg",
				"-i",
				c.AbsFile,
				"-i",
				fmt.Sprintf("streamed-%v", c.BaseFile),
				"-lavfi",
				"ssim=ssim.log;[0:v][1:v]psnr=psnr.log",
				"-f",
				"null",
				"-",
			)
			ffmpeg.Stdout = ffmpegLogFile
			ffmpeg.Stderr = ffmpegLogFile
			err = ffmpeg.Run()
			if err != nil {
				fmt.Printf("could not run ffmpeg: %v\n", err)
			}

			plotterLog := "plotter.log"
			plotterLogFile, err := os.Create(plotterLog)
			if err != nil {
				fmt.Printf("could not touch plotter log: %v", err)
				return
			}
			pyplot := exec.Command(plotter)
			pyplot.Stdout = plotterLogFile
			pyplot.Stderr = plotterLogFile
			err = pyplot.Run()
			if err != nil {
				fmt.Printf("could not run plotter: %v\n", err)
			}
		}()
	}
}

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