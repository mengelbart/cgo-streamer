
#include "gst.h"

GMainLoop *go_gst_main_loop = NULL;
void go_gst_start_mainloop(void) {
    go_gst_main_loop = g_main_loop_new(NULL, FALSE);

    g_main_loop_run(go_gst_main_loop);
}
