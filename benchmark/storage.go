package benchmark

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mengelbart/qlog"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
)

const (
	projectID            = "real-time-quic"
	experimentBucket     = "real-time-quic.appspot.com"
	experimentCollection = "experiments"
	googleAPIBaseURL     = "https://storage.googleapis.com"
)

type document struct {
	ID                string        `json:"id"`
	Filename          string        `json:"filename" firestore:"filename"`
	Bandwidth         int64         `json:"bandwidth" firestore:"bandwidth"`
	CongestionControl string        `json:"congestion_control" firestore:"congestion_control"`
	Handler           string        `json:"handler" firestore:"handler"`
	FeedbackFrequency time.Duration `json:"feedback_frequency" firestore:"feedback_frequency"`
	FeedbackAlgorithm string        `json:"feedback_algorithm" firestore:"feedback_algorithm"`
	RequestKeyFrames  bool          `json:"request_key_frames" firestore:"request_key_frames"`
	Iperf             bool          `json:"iperf" firestore:"iperf"`

	ServeCMD  string `json:"server_cmd" firestore:"server_cmd"`
	StreamCMD string `json:"client_cmd" firestore:"client_cmd"`

	Version                  string `json:"version" firestore:"version"`
	Commit                   string `json:"commit" firestore:"commit"`
	CommitTimestamp          string `json:"commit_timestamp" firestore:"commit_timestamp"`
	ExperimentStartTimestamp string `json:"experiment_start_timestamp" firestore:"experiment_start_timestamp"`
	ExperimentEndTimestamp   string `json:"experiment_end_timestamp" firestore:"experiment_end_timestamp"`

	Data map[string]string `json:"data" firestore:"data"`
}

type uploader struct {
	s *storage.Client
	f *firestore.Client

	prefix string
}

func NewUploader(prefix string) (*uploader, error) {
	ctx := context.Background()
	fs, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	experimentsIndexEntry := struct {
		Commit string `json:"commit"`
	}{
		Commit: prefix,
	}
	_, _, err = fs.Collection(experimentCollection).Add(context.Background(), experimentsIndexEntry)
	if err != nil {
		return nil, err
	}

	s, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.Bucket(experimentBucket).Update(context.Background(), storage.BucketAttrsToUpdate{CORS: []storage.CORS{{
		MaxAge:  3600,
		Methods: []string{"GET", "OPTIONS"},
		Origins: []string{"*"},
	}}}); err != nil {
		return nil, err
	}
	return &uploader{
		s:      s,
		f:      fs,
		prefix: prefix,
	}, nil
}

func (u *uploader) Close() error {
	err := u.s.Close()
	if err != nil {
		return err
	}
	return u.f.Close()
}

func (u *uploader) Upload(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	log.Printf("uploading experiment: %v\n", abs)
	e, err := parseConfig(filepath.Join(path, "config.json"))
	if err != nil {
		return err
	}
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}
	d := &document{
		ID:                       e.ID,
		Filename:                 e.BaseFile,
		Bandwidth:                int64(e.Bandwidth),
		CongestionControl:        e.CongestionControl,
		Handler:                  e.Handler,
		FeedbackFrequency:        e.FeedbackFrequency,
		FeedbackAlgorithm:        e.FeedbackAlgorithm.String(),
		RequestKeyFrames:         e.RequestKeyFrames,
		Iperf:                    e.Iperf,
		ServeCMD:                 e.ServeCMD,
		StreamCMD:                e.StreamCMD,
		Version:                  e.Version,
		Commit:                   e.Commit,
		CommitTimestamp:          e.CommitTimestamp,
		ExperimentStartTimestamp: e.ExperimentStartTimestamp,
		ExperimentEndTimestamp:   e.ExperimentEndTimestamp,
		Data:                     make(map[string]string),
	}
	expName, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("could not create uuid as exp name: %v\n", err)
	}
	for _, f := range files {
		name := f.Name()
		dataFilePath := filepath.Join(abs, name)
		convert, ok := converterMap[name]
		if !ok {
			continue
		}
		data, err := convert(dataFilePath)
		if err != nil {
			log.Printf("failed to convert file %v: %v\n", dataFilePath, err)
			continue
		}
		for name, dt := range data {
			link, err := u.store(filepath.Join(u.prefix, expName.String(), name), dt)
			if err != nil {
				log.Printf("failed to upload object %v: %v\n", dataFilePath, err)
				continue
			}
			d.Data[name] = link
		}
	}
	_, _, err = u.f.Collection(fmt.Sprintf("%v/%v/%v", experimentCollection, u.prefix, "runs")).Add(context.Background(), d)
	return err
}

func (u *uploader) store(path string, dt *DataTable) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	bkt := u.s.Bucket(experimentBucket)
	obj := bkt.Object(path + ".json")
	w := obj.NewWriter(ctx)
	defer w.Close()
	bs, err := json.Marshal(dt)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(w, bytes.NewReader(bs)); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	if err := obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
		return "", err
	}
	return fmt.Sprintf("%v/%v/%v\n", googleAPIBaseURL, obj.BucketName(), obj.ObjectName()), nil
}

func parseConfig(path string) (*experiment, error) {
	jsonFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	bs, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}
	var c experiment
	err = json.Unmarshal(bs, &c)
	if err != nil {
		return nil, err
	}
	err = jsonFile.Close()
	return &c, err
}

type Col struct {
	T     string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Row struct {
	C []Cell `json:"c"`
}

type Cell struct {
	V float64 `json:"v"`
	F string  `json:"f"`
}

type DataTable struct {
	Cols []Col `json:"cols"`
	Rows []Row `json:"rows"`
}

type converterFunc func(path string) (map[string]*DataTable, error)

func (c converterFunc) convert(path string) (map[string]*DataTable, error) {
	return c(path)
}

var converterMap = map[string]converterFunc{
	"ssim.log":           getImageMetricConverter(0, 4, "SSIM", strconv.ParseFloat),
	"psnr.log":           getImageMetricConverter(0, 5, "PSNR", parseAndBound),
	"rtcp.log":           rtcpConverter,
	"scream.log":         screamConverter,
	"server.qlog":        getQLOGConverter("server"),
	"client.qlog":        getQLOGConverter("client"),
	"server_vnstat.json": getVnstatConverter("server"),
	"client_vnstat.json": getVnstatConverter("client"),
}

func getVnstatConverter(prefix string) converterFunc {
	return func(path string) (map[string]*DataTable, error) {
		vnstatFile, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer vnstatFile.Close()

		rxBytes := &DataTable{
			Cols: []Col{
				{
					T:     "number",
					ID:    "col_1",
					Label: "n",
				},
				{
					T:     "number",
					ID:    "col_2",
					Label: fmt.Sprintf("%v RX Bytes", prefix),
				},
			},
			Rows: []Row{},
		}
		txBytes := &DataTable{
			Cols: []Col{
				{
					T:     "number",
					ID:    "col_1",
					Label: "n",
				},
				{
					T:     "number",
					ID:    "col_2",
					Label: fmt.Sprintf("%v TX Bytes", prefix),
				},
			},
			Rows: []Row{},
		}

		type x struct {
			BytesPerSecond int `json:"bytespersecond"`
		}
		type entry struct {
			Index   int `json:"index"`
			Seconds int `json:"seconds"`
			RX      *x  `json:"rx"`
			TX      *x  `json:"tx"`
		}

		scanner := bufio.NewScanner(vnstatFile)
		for scanner.Scan() {
			bs := scanner.Bytes()
			var e entry
			err := json.Unmarshal(bs, &e)
			if err != nil {
				log.Printf("skipping vnstat line: %v\n", err)
				continue
			}
			if e.RX != nil {
				rxBytes.Rows = append(rxBytes.Rows, Row{[]Cell{
					{
						V: float64(e.Seconds),
						F: fmt.Sprintf("%v", e.Seconds),
					},
					{
						V: float64(e.RX.BytesPerSecond),
						F: fmt.Sprintf("%v", e.RX.BytesPerSecond),
					},
				}})
			}
			if e.TX != nil {
				txBytes.Rows = append(txBytes.Rows, Row{[]Cell{
					{
						V: float64(e.Seconds),
						F: fmt.Sprintf("%v", e.Seconds),
					},
					{
						V: float64(e.TX.BytesPerSecond),
						F: fmt.Sprintf("%v", e.TX.BytesPerSecond),
					},
				}})
			}
		}

		return map[string]*DataTable{
			fmt.Sprintf("%v_rx_bytes", prefix): rxBytes,
			fmt.Sprintf("%v_tx_bytes", prefix): txBytes,
		}, nil
	}
}

func getQLOGConverter(prefix string) converterFunc {
	return func(path string) (map[string]*DataTable, error) {
		qlogFile, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		bs, err := ioutil.ReadAll(qlogFile)
		if err != nil {
			return nil, err
		}
		var qlogData qlog.QLOGFileNDJSON
		err = qlogData.UnmarshalNDJSON(bs)
		if err != nil {
			return nil, err
		}
		packetSent := &DataTable{
			Cols: []Col{
				{
					T:     "number",
					ID:    "col_1",
					Label: "n",
				},
				{
					T:     "number",
					ID:    "col_2",
					Label: fmt.Sprintf("%v-packet_sent_bytes", prefix),
				},
			},
			Rows: []Row{},
		}
		discretePacketSent := map[float64]*Row{}
		var discretePacketSentKeys []float64

		packetReceived := &DataTable{
			Cols: []Col{
				{
					T:     "number",
					ID:    "col_1",
					Label: "n",
				},
				{
					T:     "number",
					ID:    "col_2",
					Label: fmt.Sprintf("%v-packet_received_bytes", prefix),
				},
			},
			Rows: []Row{},
		}
		discretePacketReceived := map[float64]*Row{}
		var discretePacketReceivedKeys []float64

		for _, r := range qlogData.Trace.Events.Events {
			switch r.Name {
			case "transport:packet_sent":
				s := math.Floor(r.RelativeTime / 1000)
				if v, ok := discretePacketSent[s]; ok {
					v.C[1].V += float64(r.Data.PacketSent.Header.PacketSize)
					v.C[1].F = fmt.Sprintf("%v", v.C[1].V)
				} else {
					discretePacketSentKeys = append(discretePacketSentKeys, s)
					x := float64(r.Data.PacketSent.Header.PacketSize)
					v = &Row{[]Cell{
						{
							V: s,
							F: fmt.Sprintf("%v", s),
						},
						{
							V: x,
							F: fmt.Sprintf("%v", x),
						},
					}}
					discretePacketSent[s] = v
				}
			case "transport:packet_received":
				s := math.Floor(r.RelativeTime / 1000)
				if v, ok := discretePacketReceived[s]; ok {
					v.C[1].V += float64(r.Data.PacketReceived.Header.PacketSize)
					v.C[1].F = fmt.Sprintf("%v", v.C[1].V)
				} else {
					discretePacketReceivedKeys = append(discretePacketReceivedKeys, s)
					x := float64(r.Data.PacketReceived.Header.PacketSize)
					v = &Row{[]Cell{
						{
							V: s,
							F: fmt.Sprintf("%v", s),
						},
						{
							V: x,
							F: fmt.Sprintf("%v", x),
						},
					}}
					discretePacketReceived[s] = v
				}
			}
		}
		sort.Float64s(discretePacketSentKeys)
		for _, v := range discretePacketSentKeys {
			packetSent.Rows = append(packetSent.Rows, *discretePacketSent[v])
		}

		sort.Float64s(discretePacketReceivedKeys)
		for _, v := range discretePacketReceivedKeys {
			packetReceived.Rows = append(packetReceived.Rows, *discretePacketReceived[v])
		}
		return map[string]*DataTable{
			fmt.Sprintf("%v_packet_sent", prefix):     packetSent,
			fmt.Sprintf("%v_packet_received", prefix): packetReceived,
		}, qlogFile.Close()
	}
}

func getImageMetricConverter(first, second int, label string, parseFloat func(string, int) (float64, error)) converterFunc {
	return func(path string) (map[string]*DataTable, error) {
		csvFile, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		// defer Close for cases of early return in case of another error
		defer csvFile.Close()
		r := csv.NewReader(csvFile)
		r.Comma = ' '
		r.TrimLeadingSpace = true

		dt := &DataTable{
			Cols: []Col{
				{
					T:     "number",
					ID:    "col_1",
					Label: "n",
				},
				{
					T:     "number",
					ID:    "col_2",
					Label: label,
				},
			},
			Rows: []Row{},
		}

		for {
			record, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			nStr := strings.Split(record[first], ":")[1]
			n, err := strconv.ParseFloat(nStr, 64)
			if err != nil {
				return nil, err
			}
			vStr := strings.Split(record[second], ":")[1]
			v, err := parseFloat(vStr, 64)
			dt.Rows = append(dt.Rows, Row{[]Cell{
				{
					V: n,
					F: nStr,
				},
				{
					V: v,
					F: vStr,
				},
			}})
		}
		return map[string]*DataTable{
			label: dt,
		}, csvFile.Close()
	}
}

func parseAndBound(n string, bitSize int) (float64, error) {
	float, err := strconv.ParseFloat(n, bitSize)
	if err != nil {
		return 0, err
	}
	if math.IsInf(float, 0) {
		return 1, nil
	}
	return float / (1 + float), nil
}

func screamConverter(path string) (map[string]*DataTable, error) {
	csvFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	// defer Close for cases of early return in case of another error
	defer csvFile.Close()
	r := csv.NewReader(csvFile)
	r.Comma = ' '
	r.TrimLeadingSpace = true

	congestion := &DataTable{
		Cols: []Col{
			{
				T:     "number",
				ID:    "col_1",
				Label: "time",
			},
			{
				T:     "number",
				ID:    "col_2",
				Label: "cwnd",
			},
			{
				T:     "number",
				ID:    "col_3",
				Label: "bytesInFlight",
			},
		},
		Rows: []Row{},
	}
	bitrate := &DataTable{
		Cols: []Col{
			{
				T:     "number",
				ID:    "col_1",
				Label: "time",
			},
			{
				T:     "number",
				ID:    "col_2",
				Label: "target Bitrate",
			},
			{
				T:     "number",
				ID:    "col_3",
				Label: "rate transmitted",
			},
		},
		Rows: []Row{},
	}
	queueLength := &DataTable{
		Cols: []Col{
			{
				T:     "number",
				ID:    "col_1",
				Label: "time",
			},
			{
				T:     "number",
				ID:    "col_2",
				Label: "Queue Length",
			},
		},
		Rows: []Row{},
	}
	rtt := &DataTable{
		Cols: []Col{
			{
				T:     "numner",
				ID:    "col_1",
				Label: "time",
			},
			{
				T:     "number",
				ID:    "col_2",
				Label: "SCReAM RTT",
			},
		},
		Rows: []Row{},
	}

	// 'time', 'queueLen', 'cwnd', 'bytesInFlight', 'fastStart', 'queueDelay', 'targetBitrate', 'rateTransmitted'
	// 200		7			3525	1981				1			0.000		2048				0
	time := 0
	queueLen := 1
	rttPos := 2
	cwnd := 3
	bytesInFlight := 4
	//fastStart := 5
	//queueDelay := 6
	targetBitrate := 7
	rateTransmitted := 8
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		t, err := strconv.ParseFloat(record[time], 64)
		if err != nil {
			return nil, err
		}
		q, err := strconv.ParseFloat(record[queueLen], 64)
		if err != nil {
			return nil, err
		}
		r, err := strconv.ParseFloat(record[rttPos], 64)
		if err != nil {
			return nil, err
		}
		c, err := strconv.ParseFloat(record[cwnd], 64)
		if err != nil {
			return nil, err
		}
		b, err := strconv.ParseFloat(record[bytesInFlight], 64)
		if err != nil {
			return nil, err
		}
		br, err := strconv.ParseFloat(record[targetBitrate], 64)
		if err != nil {
			return nil, err
		}
		rt, err := strconv.ParseFloat(record[rateTransmitted], 64)
		if err != nil {
			return nil, err
		}
		congestion.Rows = append(congestion.Rows, Row{[]Cell{
			{
				V: t,
				F: record[time],
			},
			{
				V: c,
				F: record[cwnd],
			},
			{
				V: b,
				F: record[bytesInFlight],
			},
		}})
		bitrate.Rows = append(bitrate.Rows, Row{[]Cell{
			{
				V: t,
				F: record[time],
			},
			{
				V: br,
				F: record[targetBitrate],
			},
			{
				V: rt,
				F: record[rateTransmitted],
			},
		}})
		queueLength.Rows = append(queueLength.Rows, Row{[]Cell{
			{
				V: t,
				F: record[time],
			},
			{
				V: q,
				F: record[queueLen],
			},
		}})
		rtt.Rows = append(rtt.Rows, Row{[]Cell{
			{
				V: t,
				F: record[time],
			},
			{
				V: r,
				F: record[rttPos],
			},
		}})
	}
	return map[string]*DataTable{
		"scream-congestion":   congestion,
		"scream-bitrate":      bitrate,
		"scream-queue-length": queueLength,
		"scream-rtt":          rtt,
	}, csvFile.Close()
}

func rtcpConverter(path string) (map[string]*DataTable, error) {
	csvFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	// defer Close for cases of early return in case of another error
	defer csvFile.Close()
	r := csv.NewReader(csvFile)
	r.Comma = ' '
	r.TrimLeadingSpace = true

	rtcp := &DataTable{
		Cols: []Col{
			{
				T:     "number",
				ID:    "col_1",
				Label: "time",
			},
			{
				T:     "number",
				ID:    "col_2",
				Label: "rtcp",
			},
		},
		Rows: []Row{},
	}

	timeCol := 0
	rtcpCol := 1
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		t, err := strconv.ParseFloat(record[timeCol], 64)
		if err != nil {
			return nil, err
		}
		r, err := strconv.ParseFloat(record[rtcpCol], 64)
		if err != nil {
			return nil, err
		}
		rtcp.Rows = append(rtcp.Rows, Row{[]Cell{
			{
				V: t,
				F: record[timeCol],
			},
			{
				V: r,
				F: record[rtcpCol],
			},
		}})
	}
	return map[string]*DataTable{
		"rtcp-overhead": rtcp,
	}, csvFile.Close()
}
