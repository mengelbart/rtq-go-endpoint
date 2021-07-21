package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/qlog"
	"github.com/mengelbart/rtq-go"
	gstsrc "github.com/mengelbart/rtq-go-endpoint/internal/gstreamer-src"
	"github.com/mengelbart/rtq-go-endpoint/internal/utils"
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
)

func main() {
	codec := flag.String("codec", "vp8", "Video Codec")
	flag.Parse()

	logFilename := os.Getenv("LOG_FILE")
	if logFilename != "" {
		logfile, err := os.Create(logFilename)
		if err != nil {
			fmt.Printf("Could not create log file: %s\n", err.Error())
			os.Exit(1)
		}
		defer logfile.Close()
		log.SetOutput(logfile)
	}

	remoteHost := os.Getenv("RECEIVER")
	if remoteHost == "" {
		remoteHost = ":4242"
	}

	qlogWriter, err := utils.GetQLOGWriter()
	if err != nil {
		log.Printf("Could not get qlog writer: %s\n", err.Error())
		os.Exit(1)
	}

	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}

	quicConf := &quic.Config{
		EnableDatagrams: true,
	}
	if qlogWriter != nil {
		quicConf.Tracer = qlog.NewTracer(qlogWriter)
	}

	files := flag.Args()

	log.Printf("args: %v\n", files)
	src := "videotestsrc"
	if len(files) > 0 {
		src = fmt.Sprintf("filesrc location=%v ! queue ! decodebin ! videoconvert ", files[0])
	}

	err = run(remoteHost, tlsConf, quicConf, *codec, src)
	if err != nil {
		log.Printf("Could not run sender: %v\n", err.Error())
		os.Exit(1)
	}
}

type gstWriter struct {
	targetBitrate int64
	rtqSession    *rtq.Session
	pipeline      *gstsrc.Pipeline
	rtcpReader    interceptor.RTCPReader
	rtpWriter     interceptor.RTPWriter
}

func (g *gstWriter) Write(p []byte) (n int, err error) {
	var pkt rtp.Packet
	err = pkt.Unmarshal(p)
	if err != nil {
		return 0, err
	}
	n, err = g.rtpWriter.Write(&pkt.Header, p[pkt.Header.MarshalSize():], nil)
	if err != nil {
		log.Printf("failed to write paket: %v, stopping pipeline\n", err.Error())
		g.pipeline.Stop()
	}
	return
}

func (g *gstWriter) Close() error {
	return g.rtqSession.Close()
}

func run(addr string, tlsConf *tls.Config, quicConf *quic.Config, codec, src string) error {
	quicSession, err := quic.DialAddr(addr, tlsConf, quicConf)
	if err != nil {
		return err
	}
	rtqSession, err := rtq.NewSession(quicSession)
	if err != nil {
		return err
	}

	rtpFlow, err := rtqSession.OpenWriteFlow(0)
	if err != nil {
		return err
	}

	chain := interceptor.NewChain([]interceptor.Interceptor{})
	streamWriter := chain.BindLocalStream(&interceptor.StreamInfo{}, interceptor.RTPWriterFunc(func(header *rtp.Header, payload []byte, attributes interceptor.Attributes) (int, error) {
		return rtpFlow.WriteRTP(header, payload)
	}))

	writer := &gstWriter{
		rtqSession: rtqSession,
		rtpWriter:  streamWriter,
	}

	pipeline, err := gstsrc.NewPipeline(codec, src, writer)
	if err != nil {
		return err
	}
	log.Printf("created pipeline: '%v'\n", pipeline.String())
	writer.pipeline = pipeline
	pipeline.SetSSRC(0)
	pipeline.Start()

	go gstsrc.StartMainLoop()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	done := make(chan struct{}, 1)
	destroyed := make(chan struct{}, 1)
	gstsrc.HandleSinkEOS(func() {
		log.Println("got EOS, stopping pipeline")
		err := writer.Close()
		if err != nil {
			log.Printf("failed to close rtq session: %s\n", err.Error())
		}
		err = chain.Close()
		if err != nil {
			log.Printf("failed to close interceptor chain: %s\n", err.Error())
		}
		close(done)
		pipeline.Destroy()
		destroyed <- struct{}{}
	})

	select {
	case <-signals:
		log.Printf("got interrupt signal, stopping pipeline")
		pipeline.Stop()
	case <-done:
	}

	<-destroyed
	log.Println("destroyed pipeline, exiting")

	return err
}
