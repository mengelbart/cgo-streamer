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

func (p *Pipeline) Write(b []byte) (n int, err error) {
	log.Printf("writing %v bytes to pipeline", len(b))
	bs := C.CBytes(b)
	defer C.free(bs)
	C.go_gst_receive_push_buffer(p.Pipeline, bs, C.int(len(b)))
	return len(b), nil
}
