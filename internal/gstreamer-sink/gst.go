package gst

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0

#include "gst.h"

*/
import "C"
import (
	"errors"
	"unsafe"
)

var ErrUnknownCodec = errors.New("unknown codec")

// StartMainLoop starts GLib's main loop
// It needs to be called from the process' main thread
// Because many gstreamer plugins require access to the main thread
// See: https://golang.org/pkg/runtime/#LockOSThread
func StartMainLoop() {
	C.gstreamer_receive_start_mainloop()
}

type Pipeline struct {
	pipeline    *C.GstElement
	pipelineStr string
}

func NewPipeline(codecName, dst string) (*Pipeline, error) {
	pipelineStr := "appsrc name=src ! application/x-rtp"

	switch codecName {
	case "vp8":
		pipelineStr += ", encoding-name=VP8-DRAFT-IETF-01 ! rtpjitterbuffer ! queue ! rtpvp8depay ! decodebin ! " + dst

	case "vp9":
		pipelineStr += ", encoding-name=VP9-DRAFT-IETF-01 ! rtpjitterbuffer ! queue ! rtpvp9depay ! decodebin ! " + dst

	case "h264":
		pipelineStr += " ! rtpjitterbuffer ! queue ! rtph264depay ! decodebin ! " + dst

	default:
		return nil, ErrUnknownCodec
	}

	pipelineStrUnsafe := C.CString(pipelineStr)
	defer C.free(unsafe.Pointer(pipelineStrUnsafe))
	return &Pipeline{
		pipeline:    C.gstreamer_receive_create_pipeline(pipelineStrUnsafe),
		pipelineStr: pipelineStr,
	}, nil
}

func (p *Pipeline) String() string {
	return p.pipelineStr
}

// Start starts the GStreamer Pipeline
func (p *Pipeline) Start() {
	C.gstreamer_receive_start_pipeline(p.pipeline)
}

func (p *Pipeline) Stop() {
	C.gstreamer_receive_stop_pipeline(p.pipeline)
}

func (p *Pipeline) Destroy() {
	C.gstreamer_receive_destroy_pipeline(p.pipeline)
}

var eosHandler func()

func HandleSinkEOS(handler func()) {
	eosHandler = handler
}

//export goHandleReceiveEOS
func goHandleReceiveEOS() {
	eosHandler()
}

// Push pushes a buffer on the appsrc of the GStreamer Pipeline
func (p *Pipeline) Push(buffer []byte) {
	b := C.CBytes(buffer)
	defer C.free(b)
	C.gstreamer_receive_push_buffer(p.pipeline, b, C.int(len(buffer)))
}
