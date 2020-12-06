package benchmark

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gonum.org/v1/gonum/stat/combin"
)

type Bitrate uint64

const (
	BitPerSecond  Bitrate = 1
	KBitPerSecond         = 1000 * BitPerSecond
	MBitPerSecond         = 1000 * KBitPerSecond
)

type experiment struct {
	Filename          string        `json:"filename"`
	AbsFile           string        `json:"absolute_filename"`
	BaseFile          string        `json:"base_filename"`
	Bandwidth         Bitrate       `json:"bandwidth"`
	CongestionControl string        `json:"congestion_control"`
	Handler           string        `json:"handler"`
	FeedbackFrequency time.Duration `json:"feedback_frequency"`
	RequestKeyFrames  bool          `json:"request_key_frames"`
	Iperf             bool          `json:"iperf"`

	ServeCMD  string `json:"server_cmd"`
	StreamCMD string `json:"client_cmd"`

	serve  *exec.Cmd
	stream *exec.Cmd

	Version                  string `json:"version"`
	Commit                   string `json:"commit"`
	CommitTimestamp          string `json:"commit_timestamp"`
	ExperimentStartTimestamp string `json:"experiment_start_timestamp"`
	ExperimentEndTimestamp   string `json:"experiment_end_timestamp"`

	addr string
	port string

	u *uploader
}

func (e experiment) String() string {
	name := fmt.Sprintf(
		"%v-%v-%v-%v-%v",
		e.BaseFile,
		e.Handler,
		e.Bandwidth,
		e.CongestionControl,
		e.FeedbackFrequency,
	)
	if e.RequestKeyFrames {
		name = fmt.Sprintf("%v-k", name)
	} else {
		name = fmt.Sprintf("%v-nk", name)
	}
	if e.Iperf {
		name = fmt.Sprintf("%v-i", name)
	} else {
		name = fmt.Sprintf("%v-ni", name)
	}
	return name
}

func (e experiment) serveCmd() []string {
	cmd := []string{
		"serve",
		"-v",
		"-a",
		fmt.Sprintf("%v:%v", e.addr, e.port),
		"--qlog",
		"server.qlog",
		"--video-src",
		e.AbsFile,
		"--handler",
		e.Handler,
	}

	if e.CongestionControl == "scream" {
		cmd = append(cmd, "-s", "--scream-logger", "scream.log")
	}
	if e.RequestKeyFrames {
		cmd = append(cmd, "-k")
	}
	return cmd
}

func (e experiment) clientCmd() []string {
	cmd := []string{
		"stream",
		"-v",
		"-a",
		fmt.Sprintf("%v:%v", e.addr, e.port),
		"--qlog",
		"client.qlog",
		"--video-sink",
		fmt.Sprintf("streamed-%v", e.BaseFile),
		"--handler",
		e.Handler,
	}

	if e.CongestionControl == "scream" {
		cmd = append(cmd, "-s", "--feedback-frequency", fmt.Sprintf("%v", e.FeedbackFrequency.Milliseconds()))
	}
	return cmd
}

func (e *experiment) setup(binary string) error {
	// Create and change to new directory
	err := os.Mkdir(e.String(), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.Chdir(e.String())
	if err != nil {
		return err
	}

	// generate server and client commands
	serveLogFile, err := os.Create("serve.log")
	if err != nil {
		fmt.Printf("could not touch serve log: %v", err)
		return err
	}
	e.serve = exec.Command("ip", append([]string{"netns", "exec", "ns1", binary}, e.serveCmd()...)...)
	e.serve.Stdout = serveLogFile
	e.serve.Stderr = serveLogFile
	e.ServeCMD = strings.Join(append([]string{e.serve.Path}, e.serve.Args...), " ")

	clientLogFile, err := os.Create("client.log")
	if err != nil {
		fmt.Printf("could not touch client log: %v", err)
		return err
	}
	e.stream = exec.Command("ip", append([]string{"netns", "exec", "ns2", binary}, e.clientCmd()...)...)
	e.stream.Stdout = clientLogFile
	e.stream.Stderr = clientLogFile
	e.StreamCMD = strings.Join(append([]string{e.stream.Path}, e.stream.Args...), " ")

	// Write config file
	file, err := json.MarshalIndent(e, "", "	")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile("config.json", file, 0644)
	if err != nil {
		return err
	}

	if e.Bandwidth > 0 {
		err = setBandwidth(e.Bandwidth)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *experiment) Teardown() error {
	// stop server
	if err := e.serve.Process.Kill(); err != nil {
		fmt.Printf("could not kill serve cmd: %v\n", err)
		return err
	}

	ffmpegLog := "ffmpeg.log"
	ffmpegLogFile, err := os.Create(ffmpegLog)
	if err != nil {
		fmt.Printf("could not touch ffmpeg log: %v", err)
		return err
	}
	ffmpeg := exec.Command(
		"ffmpeg",
		"-i",
		e.AbsFile,
		"-i",
		fmt.Sprintf("streamed-%v", e.BaseFile),
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

	if e.u != nil {
		if err := e.u.Upload("."); err != nil {
			log.Printf("failed to upload experiment: %v\n", err)
		}
	}

	f := fmt.Sprintf("streamed-%v", e.BaseFile)
	err = os.Remove(f)
	if err != nil {
		fmt.Printf("could not remove file %v: %v\n", f, err)
	}

	if e.Bandwidth > 0 {
		err = deleteBandwidthLimit()
		if err != nil {
			return err
		}
	}

	err = os.Chdir("..")
	if err != nil {
		return err
	}
	return nil
}

func (e *experiment) Run() error {
	e.ExperimentStartTimestamp = strconv.FormatInt(time.Now().UTC().Unix(), 10)
	defer func() {
		e.ExperimentEndTimestamp = strconv.FormatInt(time.Now().UTC().Unix(), 10)
	}()
	err := e.serve.Start()
	if err != nil {
		fmt.Printf("could not run server: %v\n", err)
		return err
	}

	err = e.stream.Start()
	if err != nil {
		fmt.Printf("could not start stream client: %v\n", err)
		return err
	}

	if e.Iperf {
		iperf3Server := exec.Command(
			"ip",
			"netns",
			"exec",
			"ns1",
			"iperf3",
			"-s",
			"-B",
			e.addr,
			"--logfile",
			"iperf3server.log",
			"-J",
		)
		iperf3Server.Stdout = os.Stdout
		iperf3Server.Stderr = os.Stderr

		cancel := time.AfterFunc(15*time.Second, func() {
			iperf3Client := exec.Command(
				"ip",
				"netns",
				"exec",
				"ns2",
				"iperf3",
				"-c",
				e.addr,
				"-b",
				fmt.Sprintf("%v", e.Bandwidth/2),
				"--logfile",
				"iperf3client.log",
				"-J",
				"-t",
				"15",
			)
			iperf3Client.Stdout = os.Stdout
			iperf3Client.Stderr = os.Stderr
			err2 := iperf3Client.Run()
			if err2 != nil {
				log.Printf("failed to run iperf3 client: %v", err2)
			}
			err2 = iperf3Server.Process.Kill()
			if err2 != nil {
				log.Printf("failed to run iperf3 client: %v", err2)
			}
		})
		err = iperf3Server.Start()
		if err != nil {
			log.Printf("failed to run iperf3 server: %v", err)
			cancel.Stop()
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- e.stream.Wait()
	}()
	select {
	case <-time.After(3 * time.Minute):
		if err := e.stream.Process.Kill(); err != nil {
			fmt.Printf("could not kill process: %v\n", err)
			return err
		}
		fmt.Printf("stream client process killed after timeout:\n%v\n", e.clientCmd())
		return err
	case err := <-done:
		if err != nil {
			fmt.Printf("stream client process finished with error: %v\nconfig: %v\n", err, e)
			return err
		}
	}
	return nil
}

func setBandwidth(b Bitrate) error {
	var err error
	for i := 1; i <= 2; i++ {
		tc := exec.Command("tc", "-n", fmt.Sprintf("ns%v", i), "qdisc", "add", "dev", fmt.Sprintf("veth%v", i), "root", "netem", "rate", fmt.Sprintf("%v", b))
		tc.Stdout = os.Stdout
		tc.Stderr = os.Stderr
		err1 := tc.Run()
		if err1 != nil {
			fmt.Printf("tc add for ns%v returned error: %v\n", i, err)
			err = fmt.Errorf("%v, %v", err, err1)
		}
	}
	return err
}

func deleteBandwidthLimit() error {
	var err error
	for i := 1; i <= 2; i++ {
		tc := exec.Command("tc", "-n", fmt.Sprintf("ns%v", i), "qdisc", "delete", "dev", fmt.Sprintf("veth%v", i), "root")
		tc.Stdout = os.Stdout
		tc.Stderr = os.Stderr
		err1 := tc.Run()
		if err1 != nil {
			fmt.Printf("tc delete for ns%v returned error: %v\n", i, err)
			err = fmt.Errorf("%v, %v", err, err1)
		}
	}
	return err
}

// Evaluator runs experiments for all valid combinations of the given configurations
type Evaluator struct {
	InputFiles            []string
	Bandwidths            []Bitrate
	CongestionControllers []string
	Handlers              []string
	FeedbackFrequencies   []time.Duration
	RequestKeyFrames      []bool
	Iperf                 []bool
}

func (e *Evaluator) buildExperiments() []*experiment {
	lens := []int{
		len(e.InputFiles),
		len(e.Bandwidths),
		len(e.CongestionControllers),
		len(e.Handlers),
		len(e.Iperf),
		len(e.FeedbackFrequencies),
		len(e.RequestKeyFrames),
	}
	gen := combin.NewCartesianGenerator(lens)
	var experiments []*experiment
	for gen.Next() {
		p := gen.Product(nil)
		c := &experiment{
			Filename:          e.InputFiles[p[0]],
			Bandwidth:         e.Bandwidths[p[1]],
			CongestionControl: e.CongestionControllers[p[2]],
			Handler:           e.Handlers[p[3]],
			Iperf:             e.Iperf[p[4]],
			FeedbackFrequency: e.FeedbackFrequencies[p[5]],
			RequestKeyFrames:  e.RequestKeyFrames[p[6]],
		}
		// filter redundant none cc settings, RequestKeyFrames and FeedbackFrequency don't make sense without cc
		if c.CongestionControl == "none" && (c.RequestKeyFrames || c.FeedbackFrequency != 1*time.Millisecond) {
			continue
		}
		experiments = append(experiments, c)
	}
	return initFilePaths(experiments)
}

func initFilePaths(raw []*experiment) []*experiment {
	for _, e := range raw {
		abs, err := filepath.Abs(e.Filename)
		if err != nil {
			panic(err)
		}
		base := filepath.Base(e.Filename)
		e.AbsFile = abs
		e.BaseFile = base
	}
	return raw
}

func (e *Evaluator) RunAll(dataDir, version, commit, timestamp, addr, port string, upload bool) error {
	experiments := e.buildExperiments()

	binary, err := os.Executable()
	if err != nil {
		log.Printf("can't find executable to run: %v\n", err)
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Printf("could not get hostname, using 'unknownhost', err: %v\n", err)
		hostname = "unknownhost"
	}

	expDir := filepath.Join(dataDir, commit, hostname)
	err = os.MkdirAll(expDir, os.ModePerm)
	if err != nil {
		log.Printf("can't prepare output directory: %v\n", err)
		return err
	}

	err = os.Chdir(expDir)
	if err != nil {
		log.Printf("can't change into output directory: %v\n", err)
		return err
	}

	var u *uploader
	if upload {
		u, err = NewUploader(commit)
		if err != nil {
			return err
		}
	}

	log.Printf("running %v configs", len(experiments))
	var retries []*experiment
	for _, e := range experiments {
		e.Version = version
		e.Commit = commit
		e.CommitTimestamp = timestamp
		e.addr = addr
		e.port = port
		e.u = u
		err := e.setup(binary)
		if err != nil {
			log.Printf("failed setup experiment, queuing for retry: %v, %v\n", e, err)
			retries = append(retries, e)
			continue
		}
		err = e.Run()
		if err != nil {
			log.Printf("failed run experiment, queuing for retry: %v, %v\n", e, err)
			retries = append(retries, e)
		}
		err = e.Teardown()
		if err != nil {
			log.Printf("failed tear down experiment: %v, %v\n", e, err)
		}
	}
	for _, e := range retries {
		err := e.setup(binary)
		if err != nil {
			log.Printf("repeatedly failed to setup experiment: %v, %v\n", e, err)
			continue
		}
		err = e.Run()
		if err != nil {
			log.Printf("repeatedly failed to run experiment: %v, %v\n", e, err)
		}
		err = e.Teardown()
		if err != nil {
			log.Printf("failed tear down experiment: %v, %v\n", e, err)
		}
	}
	return nil
}
