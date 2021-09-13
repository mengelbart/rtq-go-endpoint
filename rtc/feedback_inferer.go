package rtc

import (
	"log"
	"net"
	"sort"
	"time"

	screamcgo "github.com/mengelbart/scream-go"
	"github.com/pion/interceptor"
	"github.com/pion/rtp"
)

func getNTPT0() float64 {
	now := time.Now()
	secs := now.Unix()
	usecs := now.UnixMicro() - secs*1e6
	return (float64(secs) + float64(usecs)*1e-6) - 1e-3
}

func getTimeBetweenNTP(t0 float64, tx time.Time) uint64 {
	secs := tx.Unix()
	usecs := tx.UnixMicro() - secs*1e6
	tt := (float64(secs) + float64(usecs)*1e-6) - t0
	ntp64 := uint64(tt * 65536.0)
	ntp := 0xFFFFFFFF & ntp64
	return ntp
}

type fbInferer struct {
	rtpConn  AckingRTPWriter
	rx       *screamcgo.Rx
	received chan []byte
	acked    chan ackedPkt
	rtt      time.Duration
	t0       float64
}

func newFBInferer(w AckingRTPWriter, rx *screamcgo.Rx, received chan []byte) *fbInferer {
	return &fbInferer{
		rtpConn:  w,
		rx:       rx,
		received: received,
		acked:    make(chan ackedPkt, 1000),
		t0:       getNTPT0(),
	}
}

type ackedPkt struct {
	receiveTS time.Time
	ssrc      uint32
	size      int
	seqNr     uint16
}

func (f *fbInferer) ntpTime(t time.Time) uint64 {
	return getTimeBetweenNTP(f.t0, t)
}

func (f *fbInferer) buffer(cancel chan struct{}) {
	t := time.NewTicker(10 * time.Millisecond)
	var buf []ackedPkt
	for {
		select {
		case pkt := <-f.acked:
			buf = append(buf, pkt)

		case <-t.C:
			if len(buf) == 0 {
				continue
			}
			sort.Slice(buf, func(i, j int) bool {
				return buf[i].seqNr < buf[j].seqNr
			})

			for _, pkt := range buf {
				f.rx.Receive(f.ntpTime(pkt.receiveTS), pkt.ssrc, pkt.size, pkt.seqNr, 0)
			}
			lastTS := buf[len(buf)-1].receiveTS
			buf = []ackedPkt{}

			if ok, fb := f.rx.CreateStandardizedFeedback(f.ntpTime(lastTS), true); ok {
				f.received <- fb
			}

		case <-cancel:
			return
		}
	}
}

func (f *fbInferer) rtpWriterFunc(header *rtp.Header, payload []byte, attributes interceptor.Attributes) (int, error) {
	n := header.MarshalSize() + len(payload)
	t := time.Now()
	n, err := f.rtpConn.WriteRTPNotify(header, payload, func(r bool) {
		if !r {
			return // ignore lost packets
		}
		arrivalTime := t.Add(f.rtt / 2)
		go func() {
			f.acked <- ackedPkt{
				receiveTS: arrivalTime,
				ssrc:      header.SSRC,
				size:      n,
				seqNr:     header.SequenceNumber,
			}
		}()
	})

	if err != nil {
		if netErr, ok := err.(net.Error); ok && !netErr.Temporary() || err.Error() == "Application error 0x0: eos" {
			return n, err
		}
		log.Printf("failed to write to rtpWriter: %T: %v\n", err, err)
	}

	return n, nil
}
