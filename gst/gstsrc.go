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
	"sync"
	"unsafe"
)

var srcPipelines = map[int]*SrcPipeline{}
var nextPipelineID = 0
var srcPipelinesLock sync.Mutex

type SrcPipeline struct {
	id       int
	pipeline *C.GstElement
	writer   io.Writer
}

func NewSrcPipeline(w io.Writer) *SrcPipeline {
	srcPipelinesLock.Lock()
	defer srcPipelinesLock.Unlock()
	id := nextPipelineID
	nextPipelineID++
	sp := &SrcPipeline{
		id:       id,
		pipeline: C.go_gst_create_src_pipeline(C.CString("videotestsrc ! x264enc ! rtph264pay name=rtph264pay ! appsink name=appsink")),
		writer:   w,
	}
	srcPipelines[sp.id] = sp
	return sp
}

func (p *SrcPipeline) Start() {
	C.go_gst_start_src_pipeline(p.pipeline, C.int(p.id))
}

func (p *SrcPipeline) Stop() {
	C.go_gst_stop_src_pipeline(p.pipeline)
}

func (p *SrcPipeline) Destroy() {
	C.go_gst_destroy_pipeline(p.pipeline)
}

func (p *SrcPipeline) SSRC() uint {
	return uint(C.go_gst_get_ssrc(p.pipeline))
}

func (p *SrcPipeline) SetSSRC(ssrc uint) {
	C.go_gst_set_ssrc(p.pipeline, C.uint(ssrc))
}

//export goHandlePipelineBuffer
func goHandlePipelineBuffer(buffer unsafe.Pointer, bufferLen C.int, duration C.int, pipelineID C.int) {
	srcPipelinesLock.Lock()
	srcPipeline, ok := srcPipelines[int(pipelineID)]
	srcPipelinesLock.Unlock()
	log.Printf("got buffer for id: %v", int(pipelineID))
	defer C.free(buffer)
	if !ok {
		log.Printf("no pipeline with ID %v, discarding buffer", int(pipelineID))
		return
	}

	bs := C.GoBytes(buffer, bufferLen)
	n, err := io.Copy(srcPipeline.writer, bytes.NewReader(bs))
	if n != int64(bufferLen) {
		log.Printf("different buffer size written: %v vs. %v", n, bufferLen)
	}
	if err != nil {
		log.Printf("failed to write n bytes to writer: %v", err)
	}
	log.Printf("%v bytes written to network", n)
}
