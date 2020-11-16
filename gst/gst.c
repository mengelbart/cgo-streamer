
#include "gst.h"
#include <sys/time.h>

GMainLoop *go_gst_main_loop = NULL;
void go_gst_start_mainloop(void) {
    go_gst_main_loop = g_main_loop_new(NULL, FALSE);

    g_main_loop_run(go_gst_main_loop);
}

double t0=0;

void go_gst_init_t0() {
    struct timeval tp;
    gettimeofday(&tp, NULL);
    t0 = (tp.tv_sec + tp.tv_usec*1e-6)-1e-3;
}


uint32_t go_gst_getTimeInNtp(){
    struct timeval tp;
    gettimeofday(&tp, NULL);
    double time = tp.tv_sec + tp.tv_usec*1e-6-t0;
    uint64_t ntp64 = time*65536.0;
    uint32_t ntp = 0xFFFFFFFF & ntp64;
    return ntp;
}