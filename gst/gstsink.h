#ifndef GST_SINK_H
#define GST_SINK_H

#include <gst/gst.h>

GstElement *go_gst_create_sink_pipeline(char *pipelineStr);
void go_gst_receive_push_buffer(GstElement *pipeline, void *buffer, int len);

#endif