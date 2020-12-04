module github.com/mengelbart/cgo-streamer

go 1.15

require (
	cloud.google.com/go v0.37.0
	github.com/google/uuid v1.1.2
	github.com/lucas-clemente/quic-go v0.18.0
	github.com/mengelbart/scream-go v0.0.0-20201116111338-be6d0928b775
	github.com/pion/rtcp v1.2.4
	github.com/pion/rtp v1.6.1
	github.com/spf13/cobra v1.0.0
	gonum.org/v1/gonum v0.8.1
)

replace github.com/lucas-clemente/quic-go => github.com/mengelbart/quic-go v0.7.1-0.20201107094608-fd5504eeef3c
