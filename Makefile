
VERSION := 0.0.1
BUILD := `git rev-parse --short HEAD`
TIMESTAMP := `date '+%s'`

BINARY=qrt

LDFLAGS=-ldflags "-X=github.com/mengelbart/cgo-streamer/cmd.Version=$(VERSION) -X=github.com/mengelbart/cgo-streamer/cmd.Commit=$(BUILD) -X=github.com/mengelbart/cgo-streamer/cmd.Timestamp=$(TIMESTAMP)"


all: build

build:
	go build $(LDFLAGS) -o $(BINARY) main.go

