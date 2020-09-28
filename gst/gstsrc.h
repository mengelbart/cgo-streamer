#ifndef GST_SRC_H
#define GST_SRC_H

#include <gst/gst.h>

extern void goHandlePipelineBuffer(void *buffer, int bufferLen, int samples);
void go_gst_create_src_pipeline(char *pipelineStr);

#endif