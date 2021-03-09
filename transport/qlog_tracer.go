package transport

import (
	"io"
	"log"
	"net"
	"time"

	"github.com/mengelbart/cgo-streamer/gst"

	"github.com/lucas-clemente/quic-go/logging"
	"github.com/pion/rtp"
)

type QUICTracer struct {
	ack chan []*Packet
}

func NewTracer(getLogWriter func(p logging.Perspective, connectionID []byte) io.WriteCloser) *QUICTracer {
	return &QUICTracer{
		ack: make(chan []*Packet, 1024),
	}
}

func (q *QUICTracer) GetACKChan() chan []*Packet {
	return q.ack
}

func (q *QUICTracer) TracerForConnection(p logging.Perspective, odcid logging.ConnectionID) logging.ConnectionTracer {
	ct := &ConnectionTracer{
		ack:     q.ack,
		packets: make(map[int64][]*Packet),
	}
	return ct
}

func (q QUICTracer) SentPacket(addr net.Addr, header *logging.Header, count logging.ByteCount, frames []logging.Frame) {
}

func (q QUICTracer) DroppedPacket(addr net.Addr, packetType logging.PacketType, count logging.ByteCount, reason logging.PacketDropReason) {
}

type ConnectionTracer struct {
	ack chan []*Packet

	packets      map[int64][]*Packet
	lastRTTStats *logging.RTTStats
}

func (c *ConnectionTracer) SentPacket(hdr *logging.ExtendedHeader, size logging.ByteCount, ack *logging.AckFrame, frames []logging.Frame) {

	for _, f := range frames {
		switch v := f.(type) {
		case *logging.DatagramFrame:
			var r rtp.Packet
			err := r.Unmarshal(v.Data)
			if err != nil {
				log.Printf("failed to parse data as rtcp Header: %v\n", err)
				continue
			}
			//log.Printf("%v: sent packet: %v\n", now, r.SequenceNumber)

			c.packets[int64(hdr.PacketNumber)] = append(c.packets[int64(hdr.PacketNumber)], &Packet{
				quicPacketNr: int64(hdr.PacketNumber),
				rtpSeqNr:     r.SequenceNumber,
			})
		}
	}
}

func (c *ConnectionTracer) ReceivedPacket(hdr *logging.ExtendedHeader, size logging.ByteCount, frames []logging.Frame) {

	for _, f := range frames {
		switch v := f.(type) {
		case *logging.AckFrame:
			var acks []*Packet
			for _, r := range v.AckRanges {
				for i := r.Smallest; i <= r.Largest; i++ {
					for j := range c.packets[int64(i)] {
						c.packets[int64(i)][j].ackTimestamp = gst.GetTimeInNTP()
						if c.lastRTTStats != nil {
							c.packets[int64(i)][j].smoothedRTT = c.lastRTTStats.SmoothedRTT().Seconds()
						}
						acks = append(acks, c.packets[int64(i)][j])
					}
					delete(c.packets, int64(i))
				}
			}
			if len(acks) > 0 {
				c.ack <- acks
			}
		}
	}
}

func (c *ConnectionTracer) RestoredTransportParameters(parameters *logging.TransportParameters) {
}

func (c ConnectionTracer) ReceivedVersionNegotiationPacket(header *logging.Header, numbers []logging.VersionNumber) {
}

func (c ConnectionTracer) ReceivedRetry(header *logging.Header) {
}

func (c ConnectionTracer) StartedConnection(local, remote net.Addr, version logging.VersionNumber, srcConnID, destConnID logging.ConnectionID) {
}

func (c ConnectionTracer) ClosedConnection(reason logging.CloseReason) {
}

func (c ConnectionTracer) SentTransportParameters(parameters *logging.TransportParameters) {
}

func (c ConnectionTracer) ReceivedTransportParameters(parameters *logging.TransportParameters) {
}

func (c ConnectionTracer) BufferedPacket(packetType logging.PacketType) {
}

func (c ConnectionTracer) DroppedPacket(packetType logging.PacketType, count logging.ByteCount, reason logging.PacketDropReason) {
}

func (c *ConnectionTracer) UpdatedMetrics(rttStats *logging.RTTStats, cwnd, bytesInFlight logging.ByteCount, packetsInFlight int) {
	if rttStats.SmoothedRTT() != 0 {
		c.lastRTTStats = rttStats
	}
}

func (c ConnectionTracer) LostPacket(level logging.EncryptionLevel, number logging.PacketNumber, reason logging.PacketLossReason) {
}

func (c ConnectionTracer) UpdatedCongestionState(state logging.CongestionState) {
}

func (c ConnectionTracer) UpdatedPTOCount(value uint32) {
}

func (c ConnectionTracer) UpdatedKeyFromTLS(level logging.EncryptionLevel, perspective logging.Perspective) {
}

func (c ConnectionTracer) UpdatedKey(generation logging.KeyPhase, remote bool) {
}

func (c ConnectionTracer) DroppedEncryptionLevel(level logging.EncryptionLevel) {
}

func (c ConnectionTracer) DroppedKey(generation logging.KeyPhase) {
}

func (c ConnectionTracer) SetLossTimer(timerType logging.TimerType, level logging.EncryptionLevel, time time.Time) {
}

func (c ConnectionTracer) LossTimerExpired(timerType logging.TimerType, level logging.EncryptionLevel) {
}

func (c ConnectionTracer) LossTimerCanceled() {
}

func (c ConnectionTracer) Close() {
}

func (c ConnectionTracer) Debug(name, msg string) {
}
