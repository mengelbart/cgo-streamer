#ifndef GST_SRC_H
#define GST_SRC_H

#include <gst/gst.h>

typedef struct SampleHandlerUserData {
    int pipelineId;
} SampleHandlerUserData;

extern void goHandlePipelineBuffer(void *buffer, int bufferLen, int pipelineId);
extern void goHandleSrcEOS(int pipelineId);
GstElement* go_gst_create_src_pipeline(char *pipelineStr);
void go_gst_start_src_pipeline(GstElement* pipeline, int pipelineId);
void go_gst_stop_src_pipeline(GstElement* pipeline);
void go_gst_force_key_frame(GstElement* pipeline);
void go_gst_destroy_src_pipeline(GstElement* pipeline);

unsigned int go_gst_get_ssrc(GstElement* pipeline);
void go_gst_set_ssrc(GstElement* pipeline, unsigned int ssrc);
void go_gst_set_bitrate(GstElement* pipeline, unsigned int bitrate);

#endif