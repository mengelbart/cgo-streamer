package transport

import (
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/mengelbart/scream-go"
	"github.com/pion/rtp"
)

type ScreamSendWriter struct {
	w        io.Writer
	q        *Queue
	screamTx *scream.Tx
	ssrc     uint
	packet   chan *rtp.Packet
	feedback <-chan []byte
	done     chan struct{}
}

func (s *ScreamSendWriter) Close() error {
	close(s.done)
	return nil
}

func NewScreamWriter(ssrc uint, w io.Writer, fb <-chan []byte) *ScreamSendWriter {
	queue := NewQueue()
	screamTx := scream.NewTx()
	screamTx.RegisterNewStream(queue, ssrc, 1, 1000, 2048000, 2048000000)

	return &ScreamSendWriter{
		w:        w,
		q:        queue,
		screamTx: screamTx,
		ssrc:     ssrc,
		packet:   make(chan *rtp.Packet, 1024),
		done:     make(chan struct{}, 1),
		feedback: fb,
	}
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
	ticker := time.NewTicker(2000 * time.Millisecond)
	var lastBitrate uint
	for {
		select {
		case <-ticker.C:
			kbps := s.screamTx.GetTargetBitrate(s.ssrc) / 1000
			if lastBitrate != uint(kbps) {
				lastBitrate = uint(kbps)
				setBitrate(lastBitrate)
				fmt.Printf("set bitrate to %v kbps\n", lastBitrate)
			}
		case <-s.done:
			return
		}

	}
}

func GetTimeNTP() uint {
	t := time.Now().Unix()
	ntp64 := t * 65536
	return uint(ntp64 & 0xFFFFFFFF)
}

func (s *ScreamSendWriter) Run() {
	timer := time.NewTimer(0)
	running := false
	for {
		stats := s.screamTx.GetStatistics(GetTimeNTP())
		statSlice := strings.Split(stats, ",")
		log.Printf("cwnd: %v, bytesInFlightLog: %v, fastStart: %v, queueDelay: %v, targetBitrate: %v, rateTransmitted: %v\n",
			statSlice[4], statSlice[5], statSlice[7], statSlice[8], statSlice[9], statSlice[11])
		//log.Println(statSlice)
		select {
		case packet := <-s.packet:
			s.q.Push(packet)
			log.Println("pushed packet to queue")
			s.screamTx.NewMediaFrame(GetTimeNTP(), s.ssrc, len(packet.Raw))

		case fb := <-s.feedback:
			s.screamTx.IncomingStandardizedFeedback(GetTimeNTP(), fb)

		case <-timer.C:
			running = false
		case <-s.done:
			log.Println("done, closing ScreamSendWriter")
			return
		}

		if s.q.Len() <= 0 {
			log.Println("queue empty, continue")
			continue
		}
		if running {
			log.Println("timer running, continue")
			continue
		}
		dT := s.screamTx.IsOkToTransmit(GetTimeNTP(), s.ssrc)
		if dT == -1 {
			log.Printf("not ok to transmit: send window full or no packets to transmit, waiting")
			continue
		}
		if dT > 0.001 {
			running = true
			log.Printf("waiting for send: %v", dT)
			timer = time.NewTimer(time.Duration(dT))
			continue
		}
		packet := s.q.Pop()
		bs, err := packet.Marshal()
		if err != nil {
			log.Println(err)
		}
		n, err := s.w.Write(bs)
		if err != nil {
			log.Println(err)
		}
		log.Printf("packet of %v bytes written from scream queue, len(queue)=%v", n, s.q.Len())
		dT = s.screamTx.AddTransmitted(
			GetTimeNTP(),
			uint(packet.SSRC),
			len(packet.Raw),
			uint(packet.SequenceNumber),
			packet.Marker,
		)
		if dT != -1 {
			log.Printf("after transmitted: waiting for %v", dT)
			running = true
			timer = time.NewTimer(time.Duration(dT))
		}
	}
}

type ScreamReadWriter struct {
	w          io.Writer
	screamRx   *scream.Rx
	packetChan chan *rtp.Packet
	CloseChan  chan struct{}
}

func NewScreamReadWriter(w io.Writer) *ScreamReadWriter {
	return &ScreamReadWriter{
		w:          w,
		screamRx:   scream.NewRx(1),
		packetChan: make(chan *rtp.Packet, 1024),
		CloseChan:  make(chan struct{}, 1),
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

func (s *ScreamReadWriter) Run(fbw io.Writer) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case p := <-s.packetChan:
			s.screamRx.Receive(
				GetTimeNTP(),
				nil,
				int(p.SSRC),
				len(p.Raw),
				int(p.SequenceNumber),
				0,
			)
		case <-ticker.C:
		case <-s.CloseChan:
			return
		}
		if ok, feedback := s.screamRx.CreateStandardizedFeedback(
			GetTimeNTP(),
			true,
		); ok {
			_, err := fbw.Write(feedback)
			if err != nil {
				log.Println(err)
			}
		}
	}
}
