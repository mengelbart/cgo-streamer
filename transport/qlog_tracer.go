package transport

import (
	"io"
	"log"
	"net"
	"time"

	"github.com/lucas-clemente/quic-go/logging"
	"github.com/lucas-clemente/quic-go/qlog"
	"github.com/pion/rtp"
)

type QUICTracer struct {
	qlog logging.Tracer
	ack  chan []*Packet
}

func NewTracer(getLogWriter func(p logging.Perspective, connectionID []byte) io.WriteCloser) *QUICTracer {
	return &QUICTracer{
		ack:  make(chan []*Packet, 1024),
		qlog: qlog.NewTracer(getLogWriter),
	}
}

func (q *QUICTracer) GetACKChan() chan []*Packet {
	return q.ack
}

func (q *QUICTracer) TracerForConnection(p logging.Perspective, odcid logging.ConnectionID) logging.ConnectionTracer {
	ct := &ConnectionTracer{
		qlogTracerForConnection: q.qlog.TracerForConnection(p, odcid),
		ack:                     q.ack,
		packets:                 make(map[int64][]*Packet),
	}
	return ct
}

func (q QUICTracer) SentPacket(addr net.Addr, header *logging.Header, count logging.ByteCount, frames []logging.Frame) {
	q.qlog.SentPacket(addr, header, count, frames)
}

func (q QUICTracer) DroppedPacket(addr net.Addr, packetType logging.PacketType, count logging.ByteCount, reason logging.PacketDropReason) {
	q.qlog.DroppedPacket(addr, packetType, count, reason)
}

type ConnectionTracer struct {
	qlogTracerForConnection logging.ConnectionTracer
	ack                     chan []*Packet

	packets map[int64][]*Packet
}

func (c *ConnectionTracer) SentPacket(hdr *logging.ExtendedHeader, size logging.ByteCount, ack *logging.AckFrame, frames []logging.Frame) {
	c.qlogTracerForConnection.SentPacket(hdr, size, ack, frames)

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
	c.qlogTracerForConnection.ReceivedPacket(hdr, size, frames)

	for _, f := range frames {
		switch v := f.(type) {
		case *logging.AckFrame:
			var acks []*Packet
			for _, r := range v.AckRanges {
				for i := r.Smallest; i <= r.Largest; i++ {
					acks = append(acks, c.packets[int64(i)]...)
					delete(c.packets, int64(i))
				}
			}
			if len(acks) > 0 {
				c.ack <- acks
			}
		}
	}
}

func (c ConnectionTracer) ReceivedVersionNegotiationPacket(header *logging.Header, numbers []logging.VersionNumber) {
	c.qlogTracerForConnection.ReceivedVersionNegotiationPacket(header, numbers)
}

func (c ConnectionTracer) ReceivedRetry(header *logging.Header) {
	c.qlogTracerForConnection.ReceivedRetry(header)
}

func (c ConnectionTracer) StartedConnection(local, remote net.Addr, version logging.VersionNumber, srcConnID, destConnID logging.ConnectionID) {
	c.qlogTracerForConnection.StartedConnection(local, remote, version, srcConnID, destConnID)
}

func (c ConnectionTracer) ClosedConnection(reason logging.CloseReason) {
	c.qlogTracerForConnection.ClosedConnection(reason)
}

func (c ConnectionTracer) SentTransportParameters(parameters *logging.TransportParameters) {
	c.qlogTracerForConnection.SentTransportParameters(parameters)
}

func (c ConnectionTracer) ReceivedTransportParameters(parameters *logging.TransportParameters) {
	c.qlogTracerForConnection.ReceivedTransportParameters(parameters)
}

func (c ConnectionTracer) BufferedPacket(packetType logging.PacketType) {
	c.qlogTracerForConnection.BufferedPacket(packetType)
}

func (c ConnectionTracer) DroppedPacket(packetType logging.PacketType, count logging.ByteCount, reason logging.PacketDropReason) {
	c.qlogTracerForConnection.DroppedPacket(packetType, count, reason)
}

func (c ConnectionTracer) UpdatedMetrics(rttStats *logging.RTTStats, cwnd, bytesInFlight logging.ByteCount, packetsInFlight int) {
	c.qlogTracerForConnection.UpdatedMetrics(rttStats, cwnd, bytesInFlight, packetsInFlight)
}

func (c ConnectionTracer) LostPacket(level logging.EncryptionLevel, number logging.PacketNumber, reason logging.PacketLossReason) {
	c.qlogTracerForConnection.LostPacket(level, number, reason)
}

func (c ConnectionTracer) UpdatedCongestionState(state logging.CongestionState) {
	c.qlogTracerForConnection.UpdatedCongestionState(state)
}

func (c ConnectionTracer) UpdatedPTOCount(value uint32) {
	c.qlogTracerForConnection.UpdatedPTOCount(value)
}

func (c ConnectionTracer) UpdatedKeyFromTLS(level logging.EncryptionLevel, perspective logging.Perspective) {
	c.qlogTracerForConnection.UpdatedKeyFromTLS(level, perspective)
}

func (c ConnectionTracer) UpdatedKey(generation logging.KeyPhase, remote bool) {
	c.qlogTracerForConnection.UpdatedKey(generation, remote)
}

func (c ConnectionTracer) DroppedEncryptionLevel(level logging.EncryptionLevel) {
	c.qlogTracerForConnection.DroppedEncryptionLevel(level)
}

func (c ConnectionTracer) DroppedKey(generation logging.KeyPhase) {
	c.qlogTracerForConnection.DroppedKey(generation)
}

func (c ConnectionTracer) SetLossTimer(timerType logging.TimerType, level logging.EncryptionLevel, time time.Time) {
	c.qlogTracerForConnection.SetLossTimer(timerType, level, time)
}

func (c ConnectionTracer) LossTimerExpired(timerType logging.TimerType, level logging.EncryptionLevel) {
	c.qlogTracerForConnection.LossTimerExpired(timerType, level)
}

func (c ConnectionTracer) LossTimerCanceled() {
	c.qlogTracerForConnection.LossTimerCanceled()
}

func (c ConnectionTracer) Close() {
	c.qlogTracerForConnection.Close()
}

func (c ConnectionTracer) Debug(name, msg string) {
	c.qlogTracerForConnection.Debug(name, msg)
}
