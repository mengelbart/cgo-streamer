package transport

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"strings"
	"time"

	"github.com/mengelbart/cgo-streamer/gst"

	"github.com/mengelbart/scream-go"
	"github.com/pion/rtp"
)

type ScreamSendWriter struct {
	w               io.WriteCloser
	q               *Queue
	screamTx        *scream.Tx
	screamRx        *scream.Rx
	ssrc            uint
	packet          chan *rtp.Packet
	feedback        <-chan []byte
	ack             <-chan []*Packet
	done            chan struct{}
	screamLogWriter io.Writer
	requestKeyFrame func()

	inferReceiveTime inferReceiveTimeFn
}

type Feedback struct {
	fb    []byte
	seqNr uint16
	ts    uint32
}

func (s *ScreamSendWriter) Close() error {
	close(s.done)
	return nil
}

func NewScreamWriter(ssrc uint, bitrate int, w io.WriteCloser, fb <-chan []byte, screamLogWriter io.Writer) *ScreamSendWriter {
	queue := NewQueue()
	screamTx := scream.NewTx()
	screamTx.RegisterNewStream(queue, ssrc, 1, 1000, float64(bitrate*1000), 2048000000)
	screamRx := scream.NewRx(ssrc)

	return &ScreamSendWriter{
		w:                w,
		q:                queue,
		screamTx:         screamTx,
		screamRx:         screamRx,
		ssrc:             ssrc,
		packet:           make(chan *rtp.Packet, 1024),
		done:             make(chan struct{}, 1),
		feedback:         fb,
		screamLogWriter:  screamLogWriter,
		inferReceiveTime: StaticReceiveTime,
	}
}

type inferReceiveTimeFn func(p *Packet, ts uint32) uint32

func StaticReceiveTime(p *Packet, ts uint32) uint32 {
	return uint32(math.Min(float64(ts-100), float64(p.sentTimestamp+1000)))
}

func (s *ScreamSendWriter) SetReceiveTimeInferFn(fn inferReceiveTimeFn) {
	s.inferReceiveTime = fn
}

func (s *ScreamSendWriter) SetKeyFrameRequester(requestKeyFrame func()) {
	s.requestKeyFrame = requestKeyFrame
}

func (s *ScreamSendWriter) Write(b []byte) (int, error) {
	packet := &rtp.Packet{}
	err := packet.Unmarshal(b)
	if err != nil {
		return 0, err
	}
	s.packet <- packet
	return len(b), nil
}

func (s ScreamSendWriter) RunBitrate(setBitrate func(uint)) {
	ticker := time.NewTicker(20 * time.Millisecond)
	var lastBitrate uint
	screamLogger := log.New(s.screamLogWriter, "", 0)
	start := time.Now()
	//screamLogger.Printf("len(queue) cwnd bytesInFlightLog fastStart queueDelay targetBitrate rateTransmitted")
	for {
		select {
		case <-ticker.C:
			stats := s.screamTx.GetStatistics(uint(gst.GetTimeInNTP() / 65536.0))
			statSlice := strings.Split(stats, ",")
			screamLogger.Printf("%v %v %v %v %v %v %v %v %v", time.Since(start).Milliseconds(), s.q.Len(), statSlice[3], statSlice[4], statSlice[5], statSlice[7], statSlice[8], statSlice[9], statSlice[11])
			kbps := s.screamTx.GetTargetBitrate(s.ssrc) / 1000
			//log.Printf("got scream bitrate: %v\n", kbps)
			if kbps <= 0 {
				//log.Printf("skipping setBitrate to %v\n", kbps)
				if s.requestKeyFrame != nil {
					//log.Printf("requesting new key frame")
					s.requestKeyFrame()
				}
				continue
			}
			if lastBitrate != uint(kbps) {
				lastBitrate = uint(kbps)
				setBitrate(lastBitrate)
				fmt.Printf("%v, SET BITRATE to %v\n", time.Since(start).Seconds(), lastBitrate)
			}
		case <-s.done:
			log.Println("leaving RunBitrate")
			return
		}

	}
}

func (s *ScreamSendWriter) RunReceiveFeedback() {
	gst.InitT0()
	for {
		//fmt.Printf("len(q)=%v, delay: %v\n", s.q.Len(), s.q.GetDelay(float64(gst.GetTimeInNTP())/65536))
		select {
		case packet := <-s.packet:
			now := gst.GetTimeInNTP()
			s.q.Push(&RTPQueueItem{
				Packet:    packet,
				Timestamp: float64(now) / 65536.0,
			})
			s.screamTx.NewMediaFrame(uint(now), s.ssrc, len(packet.Raw))

		case fb := <-s.feedback:
			s.screamTx.IncomingStandardizedFeedback(uint(gst.GetTimeInNTP()), fb)

		case <-s.done:
			if s.q.Len() <= 0 {
				log.Println("done, closing ScreamSendWriter")
				err := s.w.Close()
				if err != nil {
					log.Println(err)
				}
				return
			}
		default:
		}

		dT := s.screamTx.IsOkToTransmit(uint(gst.GetTimeInNTP()), s.ssrc)
		if dT != 0 {
			//if dT > 0 {
			//fmt.Printf("not ok to transmit: s.q.Len()=%v, dT:=%v\n", s.q.Len(), dT)
			//}
			continue
		}
		item := s.q.Pop()
		if item == nil {
			continue
		}
		bs, err := item.Packet.Marshal()
		if err != nil {
			log.Println(err)
		}
		_, err = s.w.Write(bs)
		if err != nil {
			log.Println(err)
		}
		//log.Printf("packet of %v bytes written from scream queue, len(queue)=%v", n, s.q.Len())
		now := gst.GetTimeInNTP() // TODO: This timestamp should be used in qlog_tracer!
		dT = s.screamTx.AddTransmitted(
			uint(now),
			uint(item.Packet.SSRC),
			len(item.Packet.Raw),
			uint(item.Packet.SequenceNumber),
			item.Packet.Marker,
		)
		//log.Printf("%v: sent %v, got dT=%v\n", now, item.Packet.SequenceNumber, dT)
		//log.Printf("transmitted seq nr: %v\n", item.Packet.SequenceNumber)
	}
}

type Packet struct {
	sentTimestamp     uint32
	inferredTimestamp uint32
	rtpSeqNr          uint16
	size              int

	quicPacketNr int64
}

func (s *ScreamSendWriter) RunInferFeedback(ackChan <-chan []*Packet) {

	sentPackets := make(map[uint16]*Packet) // rtp sequencenumber -> packet
	var nextReceiveCall []*Packet

	gst.InitT0()
	for {
		//fmt.Printf("len(q)=%v, delay: %v\n", s.q.Len(), s.q.GetDelay(float64(gst.GetTimeInNTP())/65536))
		select {
		case packet := <-s.packet:
			now := gst.GetTimeInNTP()
			s.q.Push(&RTPQueueItem{
				Packet:    packet,
				Timestamp: float64(now) / 65536.0,
			})
			s.screamTx.NewMediaFrame(uint(now), s.ssrc, len(packet.Raw))

		case ack := <-ackChan:
			for _, n := range ack {
				nextReceiveCall = append(nextReceiveCall, sentPackets[n.rtpSeqNr])
			}

		case fb := <-s.feedback:
			ts := binary.BigEndian.Uint32(fb[0:4])
			snr := binary.BigEndian.Uint16(fb[4:6])
			//log.Printf("TIMESTAMP: %v\n", ts)

			if p, ok := sentPackets[snr]; ok {
				nextReceiveCall = append(nextReceiveCall, p)
			}
			for _, p := range nextReceiveCall {
				p.inferredTimestamp = s.inferReceiveTime(p, ts)
				//log.Printf("%v got inferred Timestamp: %v\n", p.rtpSeqNr, p.inferredTimestamp)
				s.screamRx.Receive(uint(p.inferredTimestamp), nil, 1, p.size, int(p.rtpSeqNr), 0)
			}
			nextReceiveCall = []*Packet{}
			if ok, feedback := s.screamRx.CreateStandardizedFeedback(
				uint(ts),
				true,
			); ok {
				fbts := binary.BigEndian.Uint32(feedback[len(feedback)-4:])
				if fbts != ts {
					panic(fmt.Sprintf("feedback has wrong ts: %v: %v\n", fbts, feedback))
				}
				c := make([]byte, len(feedback))
				copy(c, feedback)
				s.screamTx.IncomingStandardizedFeedback(uint(gst.GetTimeInNTP()), c)
			}

		case <-s.done:
			if s.q.Len() <= 0 {
				log.Println("done, closing ScreamSendWriter")
				err := s.w.Close()
				if err != nil {
					log.Println(err)
				}
				return
			}
		default:
		}

		dT := s.screamTx.IsOkToTransmit(uint(gst.GetTimeInNTP()), s.ssrc)
		if dT != 0 {
			//if dT > 0 {
			//fmt.Printf("not ok to transmit: s.q.Len()=%v, dT:=%v\n", s.q.Len(), dT)
			//}
			continue
		}
		item := s.q.Pop()
		if item == nil {
			continue
		}
		bs, err := item.Packet.Marshal()
		if err != nil {
			log.Println(err)
		}
		_, err = s.w.Write(bs)
		if err != nil {
			log.Println(err)
		}
		//log.Printf("packet of %v bytes written from scream queue, len(queue)=%v", n, s.q.Len())
		now := gst.GetTimeInNTP() // TODO: This timestamp should be used in qlog_tracer!
		dT = s.screamTx.AddTransmitted(
			uint(now),
			uint(item.Packet.SSRC),
			len(item.Packet.Raw),
			uint(item.Packet.SequenceNumber),
			item.Packet.Marker,
		)
		sentPackets[item.Packet.SequenceNumber] = &Packet{
			sentTimestamp: now,
			size:          len(item.Packet.Raw),
			rtpSeqNr:      item.Packet.SequenceNumber,
		}
		//log.Printf("%v: sent %v, got dT=%v\n", now, item.Packet.SequenceNumber, dT)
	}
}

type ScreamReadWriter struct {
	w                     io.Writer
	screamRx              *scream.Rx
	packetChan            chan *rtp.Packet
	CloseChan             chan struct{}
	feedbackFrequency     time.Duration
	sendImmediateFeedback bool
}

func NewScreamReadWriter(w io.Writer, feedbackFrequency time.Duration, sendImmediateFeedback bool) *ScreamReadWriter {
	return &ScreamReadWriter{
		w:                     w,
		screamRx:              scream.NewRx(1),
		packetChan:            make(chan *rtp.Packet, 1024),
		CloseChan:             make(chan struct{}, 1),
		feedbackFrequency:     feedbackFrequency,
		sendImmediateFeedback: sendImmediateFeedback,
	}
}

func (s *ScreamReadWriter) Write(b []byte) (int, error) {
	packet := &rtp.Packet{}
	err := packet.Unmarshal(b)
	if err != nil {
		return 0, err
	}
	s.packetChan <- packet
	return s.w.Write(b)
}

func (s *ScreamReadWriter) RunFullFeedback(fbw io.Writer) {
	gst.InitT0()
	ticker := time.NewTicker(s.feedbackFrequency)
	defer ticker.Stop()
	for {
		select {
		case p := <-s.packetChan:
			s.screamRx.Receive(
				uint(gst.GetTimeInNTP()),
				nil,
				int(p.SSRC),
				len(p.Raw),
				int(p.SequenceNumber),
				0,
			)
			if s.sendImmediateFeedback {
				if ok, feedback := s.screamRx.CreateStandardizedFeedback(
					uint(gst.GetTimeInNTP()),
					true,
				); ok {
					var ccf CCFeedback
					err := ccf.UnmarshalBinary(feedback)
					if err != nil {
						log.Println(err)
					}
					log.Println(ccf.String())
					_, err = fbw.Write(feedback)
					if err != nil {
						log.Println(err)
					}
				}
			}
		case <-ticker.C:
			if ok, feedback := s.screamRx.CreateStandardizedFeedback(
				uint(gst.GetTimeInNTP()),
				true,
			); ok {
				var ccf CCFeedback
				err := ccf.UnmarshalBinary(feedback)
				if err != nil {
					log.Println(err)
				}
				_, err = fbw.Write(feedback)
				if err != nil {
					log.Println(err)
				}
			}
		case <-s.CloseChan:
			return
		}
	}
}

func (s *ScreamReadWriter) RunMinimalFeedback(fbw io.Writer) {
	gst.InitT0()
	ticker := time.NewTicker(s.feedbackFrequency)
	defer ticker.Stop()
	var lastSeqNr uint16
	var lastTs uint32
	for {
		select {
		case p := <-s.packetChan:
			lastSeqNr = p.SequenceNumber
			lastTs = gst.GetTimeInNTP()
			log.Printf("%v: received seqnr: %v\n", lastTs, p.SequenceNumber)
		case <-ticker.C:
			err := s.sendFeedback(fbw, lastTs, lastSeqNr)
			if err != nil {
				log.Println(err)
			}
		case <-s.CloseChan:
			return
		}
	}
}

func (s *ScreamReadWriter) sendFeedback(fbw io.Writer, ts uint32, seqNr uint16) error {
	now := gst.GetTimeInNTP()
	log.Printf("%v: sending feedback: ts=%v, seqNr=%v\n", now, ts, seqNr)
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, ts)
	if err != nil {
		return err
	}
	err = binary.Write(buf, binary.BigEndian, seqNr)
	if err != nil {
		return err
	}
	_, err = fbw.Write(buf.Bytes())
	return err
}
