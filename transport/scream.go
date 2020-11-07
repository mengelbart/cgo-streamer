package transport

import (
	"io"
	"log"
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
	ssrc            uint
	packet          chan *rtp.Packet
	feedback        <-chan []byte
	done            chan struct{}
	screamLogWriter io.Writer
}

func (s *ScreamSendWriter) Close() error {
	close(s.done)
	return nil
}

func NewScreamWriter(ssrc uint, bitrate int, w io.WriteCloser, fb <-chan []byte, screamLogWriter io.Writer) *ScreamSendWriter {
	queue := NewQueue()
	screamTx := scream.NewTx()
	screamTx.RegisterNewStream(queue, ssrc, 1, 1000, float64(bitrate*1000), 2048000000)

	return &ScreamSendWriter{
		w:               w,
		q:               queue,
		screamTx:        screamTx,
		ssrc:            ssrc,
		packet:          make(chan *rtp.Packet, 1024),
		done:            make(chan struct{}, 1),
		feedback:        fb,
		screamLogWriter: screamLogWriter,
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
	ticker := time.NewTicker(200 * time.Millisecond)
	var lastBitrate uint
	screamLogger := log.New(s.screamLogWriter, "", log.LstdFlags)
	//screamLogger.Printf("len(queue) cwnd bytesInFlightLog fastStart queueDelay targetBitrate rateTransmitted")
	for {
		select {
		case <-ticker.C:
			stats := s.screamTx.GetStatistics(uint(gst.GetTimeInNTP()))
			statSlice := strings.Split(stats, ",")
			screamLogger.Printf("%v %v %v %v %v %v %v", s.q.Len(), statSlice[4], statSlice[5], statSlice[7], statSlice[8], statSlice[9], statSlice[11])
			kbps := s.screamTx.GetTargetBitrate(s.ssrc) / 1000
			if lastBitrate != uint(kbps) {
				lastBitrate = uint(kbps)
				setBitrate(lastBitrate)
			}
		case <-s.done:
			log.Println("leaving RunBitrate")
			return
		}

	}
}

func (s *ScreamSendWriter) Run() {
	gst.InitT0()
	timer := time.NewTimer(0)
	running := false
	for {
		select {
		case packet := <-s.packet:
			s.q.Push(packet)
			s.screamTx.NewMediaFrame(uint(gst.GetTimeInNTP()), s.ssrc, len(packet.Raw))

		case fb := <-s.feedback:
			s.screamTx.IncomingStandardizedFeedback(uint(gst.GetTimeInNTP()), fb)

		case <-timer.C:
			running = false
		case <-s.done:
			if s.q.Len() <= 0 {
				log.Println("done, closing ScreamSendWriter")
				err := s.w.Close()
				if err != nil {
					log.Println(err)
				}
				return
			}
		}

		if s.q.Len() <= 0 {
			//log.Println("queue empty, continue")
			continue
		}
		if running {
			//log.Println("timer running, continue")
			continue
		}
		dT := s.screamTx.IsOkToTransmit(uint(gst.GetTimeInNTP()), s.ssrc)
		if dT == -1 {
			//log.Printf("not ok to transmit: send window full or no packets to transmit, waiting")
			continue
		}
		if dT > 0.001 {
			running = true
			//log.Printf("waiting for send: %v", dT)
			timer = time.NewTimer(time.Duration(dT))
			continue
		}
		packet := s.q.Pop()
		bs, err := packet.Marshal()
		if err != nil {
			log.Println(err)
		}
		_, err = s.w.Write(bs)
		if err != nil {
			log.Println(err)
		}
		//log.Printf("packet of %v bytes written from scream queue, len(queue)=%v", n, s.q.Len())
		dT = s.screamTx.AddTransmitted(
			uint(gst.GetTimeInNTP()),
			uint(packet.SSRC),
			len(packet.Raw),
			uint(packet.SequenceNumber),
			packet.Marker,
		)
		if dT != -1 {
			//log.Printf("after transmitted: waiting for %v", dT)
			running = true
			timer = time.NewTimer(time.Duration(dT))
		}
	}
}

type ScreamReadWriter struct {
	w                 io.Writer
	screamRx          *scream.Rx
	packetChan        chan *rtp.Packet
	CloseChan         chan struct{}
	feedbackFrequency time.Duration
}

func NewScreamReadWriter(w io.Writer, feedbackFrequency time.Duration) *ScreamReadWriter {
	return &ScreamReadWriter{
		w:                 w,
		screamRx:          scream.NewRx(1),
		packetChan:        make(chan *rtp.Packet, 1024),
		CloseChan:         make(chan struct{}, 1),
		feedbackFrequency: feedbackFrequency,
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
		case <-ticker.C:
		case <-s.CloseChan:
			return
		}
		if ok, feedback := s.screamRx.CreateStandardizedFeedback(
			uint(gst.GetTimeInNTP()),
			true,
		); ok {
			_, err := fbw.Write(feedback)
			if err != nil {
				log.Println(err)
			}
		}
	}
}
