package gst

/*
#cgo pkg-config: gstreamer-1.0

#include "gstsrc.h"

*/
import "C"
import (
	"bytes"
	"io"
	"log"
	"unsafe"
)

var writer io.Writer

func CreateSrcPipeline(w io.Writer) {
	writer = w
	C.go_gst_create_src_pipeline(C.CString("videotestsrc ! x264enc ! rtph264pay ! appsink name=appsink"))
}

//export goHandlePipelineBuffer
func goHandlePipelineBuffer(buffer unsafe.Pointer, bufferLen C.int, duration C.int) {
	bs := C.GoBytes(buffer, bufferLen)
	n, err := io.Copy(writer, bytes.NewReader(bs))
	if n != int64(bufferLen) {
		log.Printf("different buffer size written: %v vs. %v", n, bufferLen)
	}
	if err != nil {
		log.Printf("failed to write n bytes to writer: %v", err)
	}
	log.Printf("%v bytes written to network", n)
	C.free(buffer)
}
