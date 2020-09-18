package gstsrc

/*
#cgo pkg-config: gstreamer-1.0

#include "gst.h"

*/
import "C"
import (
	"io"
	"log"
	"unsafe"
)

func init() {
	go C.go_gst_start_mainloop()
}

var writer io.Writer

func CreatePipeline(w io.Writer) {
	writer = w
	//C.go_gst_create_pipeline(C.CString("videotestsrc ! video/x-raw,format=I420 ! x264enc ! video/x-h264,stream-format=byte-stream ! appsink max-buffers=1 drop=TRUE name=appsink"))
	C.go_gst_create_pipeline(C.CString("videotestsrc ! video/x-raw,format=I420 ! x264enc ! video/x-h264,stream-format=byte-stream ! appsink name=appsink"))
}

var numBytes = 0

//export goHandlePipelineBuffer
func goHandlePipelineBuffer(buffer unsafe.Pointer, bufferLen C.int, duration C.int) {
	log.Printf("trying to write bytes to buffer")
	bs := C.GoBytes(buffer, bufferLen)
	n, err := writer.Write(bs)
	if err != nil {
		log.Printf("failed to n buffer to writer: %v", err)
	}
	numBytes += n
	C.free(buffer)
	log.Printf("%v bytes written to writer, total: %v", n, numBytes)
}
