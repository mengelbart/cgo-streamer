#ifndef GST_H
#define GST_H

#include <gst/gst.h>

extern void goHandlePipelineBuffer(void *buffer, int bufferLen, int samples);
void go_gst_create_pipeline(char *pipelineStr);

void go_gst_start_mainloop(void);

#endif