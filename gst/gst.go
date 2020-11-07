package gst

/*
#cgo pkg-config: gstreamer-1.0

#include "gst.h"
*/
import "C"

func StartMainLoop() {
	go C.go_gst_start_mainloop()
}

func GetTimeInNTP() uint32 {
	return uint32(C.go_gst_getTimeInNtp())
}

func InitT0() {
	C.go_gst_init_t0()
}
