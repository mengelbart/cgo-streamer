module github.com/mengelbart/cgo-streamer

go 1.15

require (
	github.com/lucas-clemente/quic-go v0.18.0
	github.com/mengelbart/scream-go v0.0.0-00010101000000-000000000000
	github.com/pion/rtcp v1.2.4
	github.com/pion/rtp v1.6.1
	github.com/spf13/cobra v1.0.0
)

replace github.com/lucas-clemente/quic-go => ../quic-go

replace github.com/mengelbart/scream-go => ../scream-go
