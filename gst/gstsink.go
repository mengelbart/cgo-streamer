package gst

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0

#include "gstsink.h"
*/
import "C"
import (
	"log"
)

func CreateSinkPipeline(videoSink string) *SinkPipeline {
	pipelineStr := "appsrc name=src ! application/x-rtp ! rtpjitterbuffer ! queue ! rtph264depay ! h264parse ! avdec_h264 ! " + videoSink
	log.Printf("creating pipeline: '%v'\n", pipelineStr)
	return &SinkPipeline{
		pipeline: C.go_gst_create_sink_pipeline(C.CString(pipelineStr)),
	}
}

type SinkPipeline struct {
	pipeline *C.GstElement
}

var numBytes = 0

func (p *SinkPipeline) Start() {
	C.go_gst_start_sink_pipeline(p.pipeline)
}

func (p *SinkPipeline) Stop() {
	C.go_gst_stop_sink_pipeline(p.pipeline)
}

func (p *SinkPipeline) Destroy() {
	C.go_gst_destroy_sink_pipeline(p.pipeline)
}

var eosHandler func()

func HandleSinkEOS(handler func()) {
	eosHandler = handler
}

//export goHandleSinkEOS
func goHandleSinkEOS() {
	eosHandler()
}

var countSink = 0

func (p *SinkPipeline) Write(buffer []byte) (n int, err error) {
	countSink++
	//log.Printf("%v: writing %v bytes to pipeline\n", countSink, len(buffer))
	b := C.CBytes(buffer)
	defer C.free(b)
	C.go_gst_receive_push_buffer(p.pipeline, b, C.int(len(buffer)))
	numBytes += len(buffer)
	return len(buffer), nil
}
