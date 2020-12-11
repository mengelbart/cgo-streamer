package benchmark

import (
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
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

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
	Filename          string        `json:"filename" firestore:"filename"`
	Bandwidth         int64         `json:"bandwidth" firestore:"bandwidth"`
	CongestionControl string        `json:"congestion_control" firestore:"congestion_control"`
	Handler           string        `json:"handler" firestore:"handler"`
	FeedbackFrequency time.Duration `json:"feedback_frequency" firestore:"feedback_frequency"`
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
		Filename:                 e.BaseFile,
		Bandwidth:                int64(e.Bandwidth),
		CongestionControl:        e.CongestionControl,
		Handler:                  e.Handler,
		FeedbackFrequency:        e.FeedbackFrequency,
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
	_, _, err = u.f.Collection(experimentCollection).Add(context.Background(), d)
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
	"ssim.log":   getImageMetricConverter(0, 4, "SSIM", strconv.ParseFloat),
	"psnr.log":   getImageMetricConverter(0, 5, "PSNR", parseAndBound),
	"rtcp.log":   rtcpConverter,
	"scream.log": screamConverter,
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

	// 'time', 'queueLen', 'cwnd', 'bytesInFlight', 'fastStart', 'queueDelay', 'targetBitrate', 'rateTransmitted'
	// 200		7			3525	1981				1			0.000		2048				0
	time := 0
	queueLen := 1
	cwnd := 2
	bytesInFlight := 3
	//fastStart := 4
	//queueDelay := 5
	targetBitrate := 6
	rateTransmitted := 7
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
	}
	return map[string]*DataTable{
		"scream-congestion":   congestion,
		"scream-bitrate":      bitrate,
		"scream-queue-length": queueLength,
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
