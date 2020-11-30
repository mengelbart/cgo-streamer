package benchmark

import "time"

type config struct {
	Filename          string        `json:"filename"`
	AbsFile           string        `json:"absolute_filename"`
	BaseFile          string        `json:"base_filename"`
	Bandwidth         bitrate       `json:"bandwidth"`
	CongestionControl string        `json:"congestion_control"`
	Handler           string        `json:"handler"`
	FeedbackFrequency time.Duration `json:"feedback_frequency"`

	Version string `json:"version"`
}

type bitrate uint64
