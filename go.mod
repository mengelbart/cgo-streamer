module github.com/mengelbart/cgo-streamer

go 1.15

require (
	github.com/lucas-clemente/quic-go v0.18.0
	github.com/mengelbart/scream-go v0.0.0-20201112103702-e3ca855ff2d4
	github.com/pion/rtcp v1.2.4
	github.com/pion/rtp v1.6.1
	github.com/spf13/cobra v1.0.0
	gonum.org/v1/gonum v0.8.1
)

replace github.com/lucas-clemente/quic-go => github.com/mengelbart/quic-go v0.7.1-0.20201107094608-fd5504eeef3c
