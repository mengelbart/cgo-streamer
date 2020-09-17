package gstsink

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0

#include "gst.h"
*/
import "C"
import "log"

func init() {
	go C.go_gst_start_mainloop()
}

func CreatePipeline() *Pipeline {
	return &Pipeline{
		Pipeline: C.go_gst_create_pipeline(C.CString("appsrc name=src ! decodebin ! autovideosink")),
	}
}

type Pipeline struct {
	Pipeline *C.GstElement
}

var numBytes = 0

func (p *Pipeline) Write(b []byte) (n int, err error) {
	log.Println("trying to write bytes to pipeline")
	bs := C.CBytes(b)
	defer C.free(bs)
	C.go_gst_receive_push_buffer(p.Pipeline, bs, C.int(len(b)))
	numBytes += len(b)
	log.Printf("%v bytes written to pipeline, total: %v", len(b), numBytes)
	return len(b), nil
}
