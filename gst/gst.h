#ifndef GST_H
#define GST_H

#include <gst/gst.h>
#include <stdint.h>

void go_gst_start_mainloop(void);
void go_gst_init_t0();
uint32_t go_gst_getTimeInNtp();

#endif