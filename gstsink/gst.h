#ifndef GST_H
#define GST_H

#include <gst/gst.h>

GstElement *go_gst_create_pipeline(char *pipelineStr);
void go_gst_receive_push_buffer(GstElement *pipeline, void *buffer, int len);

void go_gst_start_mainloop(void);

#endif