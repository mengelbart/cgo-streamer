package gst

/*
#cgo pkg-config: gstreamer-1.0

#include "gst.h"
*/
import "C"

func StartMainLoop() {
	go C.go_gst_start_mainloop()
}
