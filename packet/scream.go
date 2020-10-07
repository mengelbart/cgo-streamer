package packet

import (
	"io"
	"log"
	"time"

	"github.com/mengelbart/scream-go"
	"github.com/pion/rtp"
)

type ScreamWriter struct {
	w        io.Writer
	q        *Queue
	screamTx *scream.Tx
	ssrc     uint
	packet   chan *rtp.Packet
	feedback chan []byte
}

func NewScreamWriter(ssrc uint, w io.Writer, fb chan []byte) *ScreamWriter {
	queue := &Queue{}
	screamTx := scream.NewTx()
	screamTx.RegisterNewStream(queue, ssrc, 1, 1000, 2048000, 2048000000)

	return &ScreamWriter{
		w:        w,
		q:        queue,
		screamTx: screamTx,
		ssrc:     ssrc,
		packet:   make(chan *rtp.Packet, 1024),
		feedback: fb,
	}
}

func (s *ScreamWriter) Write(b []byte) (int, error) {
	packet := &rtp.Packet{}
	err := packet.Unmarshal(b)
	if err != nil {
		return 0, err
	}
	s.packet <- packet
	return len(b), nil
}

func (s ScreamWriter) RunBitrate(done chan struct{}, setBitrate func(uint)) {
	ticker := time.NewTicker(200 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
			kbps := s.screamTx.GetTargetBitrate(s.ssrc) / 1000
			setBitrate(uint(kbps))
			log.Printf("set bitrate to %v kbps", kbps)
		case <-done:
			return
		}

	}
}

func (s *ScreamWriter) Run() {
	timer := time.NewTimer(0)
	running := false
	for {
		select {
		case packet := <-s.packet:
			s.q.Push(packet)
			s.screamTx.NewMediaFrame(uint(time.Now().UTC().Unix()), s.ssrc, len(packet.Raw))
			if running {
				continue
			}

		case fb := <-s.feedback:
			fbPacket := &CCFeedback{}
			err := fbPacket.UnmarshalBinary(fb)
			if err != nil {
				log.Println(err)
			}
			s.screamTx.IncomingStandardizedFeedback(uint(time.Now().UTC().Unix()), fb)
			if running {
				continue
			}

		case <-timer.C:
			running = false
		}

		dT := s.screamTx.IsOkToTransmit(uint(time.Now().UTC().Unix()), s.ssrc)
		if dT == -1 {
			log.Println("send window full, waiting")
			continue
		}
		if dT > 0.001 {
			running = true
			log.Printf("waiting for send: %v", dT)
			timer = time.NewTimer(time.Duration(dT))
			continue
		}
		packet := s.q.Pop()
		if packet == nil {
			continue
		}
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
			uint(time.Now().UTC().Unix()),
			uint(packet.SSRC),
			len(packet.Raw),
			uint(packet.SequenceNumber),
			packet.Marker,
		)
		if dT == -1 {
			log.Println("send window full, waiting")
			continue
		}

		running = true
		timer = time.NewTimer(time.Duration(dT))
	}
}
