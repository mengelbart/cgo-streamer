package transport

import "io"

type SrcFactory interface {
	MakeSrc(writer io.WriteCloser, feedback <-chan []byte) func()
}
