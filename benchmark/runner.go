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
	ID                string        `json:"id"`
	Filename          string        `json:"filename"`
	AbsFile           string        `json:"absolute_filename"`
	BaseFile          string        `json:"base_filename"`
	Bandwidth         Bitrate       `json:"bandwidth"`
	CongestionControl string        `json:"congestion_control"`
	Handler           string        `json:"handler"`
	FeedbackFrequency time.Duration `json:"feedback_frequency"`
	RequestKeyFrames  bool          `json:"request_key_frames"`
	Iperf             bool          `json:"iperf"`
	FeedbackAlgorithm int           `json:"feedback_algorithm"`

	ServeCMD  string `json:"server_cmd"`
	StreamCMD string `json:"client_cmd"`

	serve  *exec.Cmd
	stream *exec.Cmd

	serverVnstat *exec.Cmd
	clientVnstat *exec.Cmd

	files []*os.File

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

func (e experiment) serveIperf() []string {
	cmd := []string{
		"-s",
		"-p",
		"3000",
	}
	return cmd
}

func (e experiment) clientIperf() []string {
	cmd := []string{
		"-c",
		fmt.Sprintf("%v", e.addr),
		"-p",
		"3000",
		"-u",
		"-b",
		"100mbit",
		"-R",
		"-t",
		"30",
	}
	return cmd
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
		"--feedback-algorithm",
		fmt.Sprintf("%v", e.FeedbackAlgorithm),
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
		"--feedback-algorithm",
		fmt.Sprintf("%v", e.FeedbackAlgorithm),
	}

	if e.CongestionControl == "scream" {
		cmd = append(
			cmd,
			"-s",
			"--feedback-frequency",
			fmt.Sprintf("%v", e.FeedbackFrequency.Milliseconds()),
			"--rtcp-logger",
			"rtcp.log",
		)
	}
	return cmd
}

func (e *experiment) setup(binary string) error {
	// Create and change to new directory
	dirName := fmt.Sprintf("%v", e.ID)
	if _, err := os.Stat(dirName); !os.IsNotExist(err) {
		err := os.RemoveAll(dirName)
		if err != nil {
			return err
		}
	}
	err := os.Mkdir(dirName, os.ModePerm)
	if err != nil {
		return err
	}
	err = os.Chdir(dirName)
	if err != nil {
		return err
	}

	// generate server and client commands
	serveLogFile, err := os.Create("serve.log")
	if err != nil {
		fmt.Printf("could not touch serve log: %v", err)
		return err
	}
	e.files = append(e.files, serveLogFile)
	e.serve = exec.Command("ip", append([]string{"netns", "exec", "ns1", binary}, e.serveCmd()...)...)
	//e.serve = exec.Command("ip", append([]string{"netns", "exec", "ns1", "iperf3"}, e.serveIperf()...)...)
	e.serve.Stdout = serveLogFile
	e.serve.Stderr = serveLogFile
	e.ServeCMD = strings.Join(append([]string{e.serve.Path}, e.serve.Args...), " ")

	clientLogFile, err := os.Create("client.log")
	if err != nil {
		fmt.Printf("could not touch client log: %v", err)
		return err
	}
	e.files = append(e.files, clientLogFile)
	e.stream = exec.Command("ip", append([]string{"netns", "exec", "ns2", binary}, e.clientCmd()...)...)
	//e.stream = exec.Command("ip", append([]string{"netns", "exec", "ns2", "iperf3"}, e.clientIperf()...)...)
	e.stream.Stdout = clientLogFile
	e.stream.Stderr = clientLogFile
	e.StreamCMD = strings.Join(append([]string{e.stream.Path}, e.stream.Args...), " ")

	// setup vnstat commands
	serverVnstatLogFile, err := os.Create("server_vnstat.json")
	if err != nil {
		fmt.Printf("could not touch server vnstat log file: %v", err)
		return err
	}
	e.files = append(e.files)
	e.serverVnstat = exec.Command("ip", "netns", "exec", "ns1", "vnstat", "-l", "-i", "veth1", "--json")
	e.serverVnstat.Stdout = serverVnstatLogFile
	e.serverVnstat.Stderr = serverVnstatLogFile

	clientVnstatLogFile, err := os.Create("client_vnstat.json")
	if err != nil {
		fmt.Printf("could not touch client vnstat log file: %v", err)
		return err
	}
	e.files = append(e.files)
	e.clientVnstat = exec.Command("ip", "netns", "exec", "ns2", "vnstat", "-l", "-i", "veth2", "--json")
	e.clientVnstat.Stdout = clientVnstatLogFile
	e.clientVnstat.Stderr = clientVnstatLogFile

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

	if err := e.serverVnstat.Process.Kill(); err != nil {
		fmt.Printf("could not kill server vnstat cmd: %v\n", err)
		return err
	}
	if err := e.clientVnstat.Process.Kill(); err != nil {
		fmt.Printf("could not kill client vnstat cmd: %v\n", err)
		return err
	}

	ffmpegLog := "ffmpeg.log"
	ffmpegLogFile, err := os.Create(ffmpegLog)
	if err != nil {
		fmt.Printf("could not touch ffmpeg log: %v", err)
		return err
	}
	e.files = append(e.files, ffmpegLogFile)
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

	for _, f := range e.files {
		err = f.Close()
		if err != nil {
			log.Printf("failed to close file: %v\n", f.Name())
		}
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

	time.Sleep(2 * time.Second)

	if err = e.clientVnstat.Start(); err != nil {
		fmt.Printf("could not start client vnstat: %v\n", err)
		return err
	}
	if err = e.serverVnstat.Start(); err != nil {
		fmt.Printf("could not start server vnstat: %v\n", err)
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

		cancel := time.AfterFunc(2*time.Minute, func() {
			iperf3Client := exec.Command(
				"ip",
				"netns",
				"exec",
				"ns2",
				"iperf3",
				"-u",
				"-c",
				e.addr,
				"-b",
				fmt.Sprintf("%v", e.Bandwidth/2),
				"--logfile",
				"iperf3client.log",
				"-J",
				"-t",
				"60",
				"-R",
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
	case <-time.After(5 * time.Minute):
		if err := e.stream.Process.Kill(); err != nil {
			log.Printf("could not kill process: %v\n", err)
			return err
		}
		log.Printf("stream client process killed after timeout:\n%v\n", e.clientCmd())
		return nil
	case err := <-done:
		if err != nil {
			log.Printf("stream client process finished with error: %v\nconfig: %v\n", err, e)
			return err
		}
	}
	return nil
}

func setBandwidth(b Bitrate) error {
	var err error
	for i := 1; i <= 2; i++ {
		tc := exec.Command("tc", "-n", fmt.Sprintf("ns%v", i), "qdisc", "add", "dev", fmt.Sprintf("veth%v", i), "root", "tbf", "rate", fmt.Sprintf("%v", b), "limit", "100kB", "burst", "100kB")
		fmt.Printf("%v %v\n", tc.Path, tc.Args)
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
		fmt.Printf("%v %v\n", tc.Path, tc.Args)
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
	FeedbackAlgorithms    []int
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
		len(e.FeedbackAlgorithms),
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
			FeedbackAlgorithm: e.FeedbackAlgorithms[p[7]],
		}
		// filter redundant none cc settings, RequestKeyFrames and FeedbackFrequency don't make sense without cc
		if c.CongestionControl == "none" && (c.RequestKeyFrames || c.FeedbackFrequency != 1*time.Millisecond) {
			continue
		}
		// filter inferred feedback for non-datagram handlers
		if c.FeedbackAlgorithm != 0 && (c.CongestionControl != "scream" || c.Handler != "datagram") {
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

	envScript, err := filepath.Abs("./vnetns.sh")
	if err != nil {
		return err
	}

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

	expDir, err := filepath.Abs(filepath.Join(dataDir, commit, hostname))
	if err != nil {
		return err
	}
	err = os.MkdirAll(expDir, os.ModePerm)
	if err != nil {
		log.Printf("can't prepare output directory: %v\n", err)
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
	var failed []struct {
		err error
		ex  *experiment
	}
	for i, e := range experiments {

		err = os.Chdir(expDir)
		if err != nil {
			log.Printf("can't change into output directory: %v\n", err)
			return err
		}

		// clean up environment from previous runs (network ns+interfaces)
		down := exec.Command(envScript, "down")
		down.Stderr = os.Stderr
		_ = down.Run()

		time.Sleep(1 * time.Second)

		up := exec.Command(envScript, "up")
		up.Stderr = os.Stderr
		err := up.Run()
		if err != nil {
			return err
		}

		e.ID = fmt.Sprintf("%v", i)
		e.Version = version
		e.Commit = commit
		e.CommitTimestamp = timestamp
		e.addr = addr
		e.port = port
		e.u = u

		err = e.setup(binary)
		if err != nil {
			log.Printf("failed setup experiment: %v, %v\n", e, err)
			failed = append(failed, struct {
				err error
				ex  *experiment
			}{err: err, ex: e})
			continue
		}
		err = e.Run()
		if err != nil {
			log.Printf("failed run experiment: %v, %v\n", e, err)
			failed = append(failed, struct {
				err error
				ex  *experiment
			}{err: err, ex: e})
		}
		err = e.Teardown()
		if err != nil {
			log.Printf("failed tear down experiment: %v, %v\n", e, err)
		}
		time.Sleep(15 * time.Second)
	}
	log.Println("finished evaluation, failed to run the following experiments:")
	for _, e := range failed {
		log.Printf("%v, err: %v\n", e.ex, e.err)
	}
	return nil
}
