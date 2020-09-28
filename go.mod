module github.com/mengelbart/cgo-streamer

go 1.15

require (
	github.com/lucas-clemente/quic-go v0.18.0
	github.com/pion/rtp v1.6.1
	github.com/spf13/cobra v1.0.0
)

replace github.com/lucas-clemente/quic-go => ../quic-go

replace github.com/mengelbart/scream-go => ../scream-go
