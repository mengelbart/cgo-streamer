package gst

/*
#cgo pkg-config: gstreamer-1.0

#include "gstsrc.h"

*/
import "C"
import (
	"bytes"
	"fmt"
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
	writer   io.WriteCloser
}

func NewSrcPipeline(w io.WriteCloser, src string, bitrate int) *SrcPipeline {
	srcPipelinesLock.Lock()
	defer srcPipelinesLock.Unlock()
	id := nextPipelineID
	nextPipelineID++
	pipelineStr := src + fmt.Sprintf(" ! x264enc name=x264enc pass=5 speed-preset=4 bitrate=%v tune=4 ! rtph264pay name=rtph264pay mtu=1000 ! appsink name=appsink", bitrate)
	log.Printf("creating pipeline: '%v'\n", pipelineStr)
	sp := &SrcPipeline{
		id:       id,
		pipeline: C.go_gst_create_src_pipeline(C.CString(pipelineStr)),
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

func (p *SrcPipeline) ForceKeyFrame() {
	C.go_gst_force_key_frame(p.pipeline)
}

func (p *SrcPipeline) Destroy() {
	C.go_gst_destroy_src_pipeline(p.pipeline)
}

func (p *SrcPipeline) SSRC() uint {
	return uint(C.go_gst_get_ssrc(p.pipeline))
}

func (p *SrcPipeline) SetSSRC(ssrc uint) {
	C.go_gst_set_ssrc(p.pipeline, C.uint(ssrc))
}

func (p *SrcPipeline) SetBitRate(bitrate uint) {
	C.go_gst_set_bitrate(p.pipeline, C.uint(bitrate))
}

var countSrc = 0

//export goHandlePipelineBuffer
func goHandlePipelineBuffer(buffer unsafe.Pointer, bufferLen C.int, pipelineID C.int) {
	srcPipelinesLock.Lock()
	srcPipeline, ok := srcPipelines[int(pipelineID)]
	srcPipelinesLock.Unlock()
	defer C.free(buffer)
	if !ok {
		log.Printf("no pipeline with ID %v, discarding buffer", int(pipelineID))
		return
	}

	bs := C.GoBytes(buffer, bufferLen)
	countSrc++
	//log.Printf("%v: writing %v bytes to conn\n", countSrc, len(bs))
	n, err := io.Copy(srcPipeline.writer, bytes.NewReader(bs))
	if n != int64(bufferLen) {
		log.Printf("different buffer size written: %v vs. %v", n, bufferLen)
	}
	if err != nil {
		log.Printf("failed to write %v bytes to writer: %v", n, err)
	}
}

//export goHandleSrcEOS
func goHandleSrcEOS(pipelineID C.int) {
	srcPipelinesLock.Lock()
	srcPipeline, ok := srcPipelines[int(pipelineID)]
	srcPipelinesLock.Unlock()
	if !ok {
		log.Printf("no pipeline with ID %v, discarding eos", int(pipelineID))
		return
	}
	err := srcPipeline.writer.Close()
	if err != nil {
		log.Printf("failed close writer with id %v: %v", int(pipelineID), err)
	}
}
