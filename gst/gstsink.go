package gst

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0

#include "gstsink.h"
*/
import "C"
import (
	"log"
)

func CreateSinkPipeline() *Pipeline {
	return &Pipeline{
		Pipeline: C.go_gst_create_sink_pipeline(C.CString("appsrc name=src ! application/x-rtp,clock-rate=90000,payload=96 ! rtpjitterbuffer ! rtph264depay ! h264parse ! avdec_h264 ! videoconvert ! autovideosink")),
	}
}

type Pipeline struct {
	Pipeline *C.GstElement
}

var numBytes = 0

func (p *Pipeline) Write(buffer []byte) (n int, err error) {
	b := C.CBytes(buffer)
	defer C.free(b)
	C.go_gst_receive_push_buffer(p.Pipeline, b, C.int(len(buffer)))
	numBytes += len(buffer)
	log.Printf("%v bytes written to pipeline", len(buffer))
	return len(buffer), nil
}
