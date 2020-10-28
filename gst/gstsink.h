#ifndef GST_SINK_H
#define GST_SINK_H

#include <gst/gst.h>

extern void goHandleSinkEOS();

GstElement *go_gst_create_sink_pipeline(char *pipelineStr);
void go_gst_start_sink_pipeline(GstElement* pipeline);
void go_gst_stop_sink_pipeline(GstElement* pipeline);
void go_gst_destroy_sink_pipeline(GstElement* pipeline);
void go_gst_receive_push_buffer(GstElement *pipeline, void *buffer, int len);

#endif