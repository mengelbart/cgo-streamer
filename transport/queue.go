package transport

import (
	"github.com/pion/rtp"
)

type Queue struct {
	q []*rtp.Packet
}

func (q *Queue) Push(p *rtp.Packet) {
	q.q = append(q.q, p)
}

func (q *Queue) Pop() *rtp.Packet {
	if len(q.q) <= 0 {
		return nil
	}
	p := q.q[0]
	q.q = q.q[1:]
	return p
}

func (q *Queue) Len() int {
	return len(q.q)
}

func (q *Queue) Clear() {
	q.q = []*rtp.Packet{}
}

func (q *Queue) SizeOfNextRTP() int {
	if len(q.q) <= 0 {
		return 0
	}
	return len(q.q[0].Raw)
}

func (q *Queue) SeqNrOfNextRTP() int {
	if len(q.q) <= 0 {
		return 0
	}
	return int(q.q[0].SequenceNumber)
}

func (q *Queue) BytesInQueue() int {
	size := 0
	for _, p := range q.q {
		size += len(p.Raw)
	}
	return size
}

func (q *Queue) SizeOfQueue() int {
	return len(q.q)
}

func (q *Queue) GetDelay(f float64) float64 {
	if len(q.q) <= 0 {
		return 0
	}
	return f - float64(q.q[0].Timestamp)
}

// TODO: Which frame?
func (q *Queue) GetSizeOfLastFrame() int {
	if len(q.q) <= 0 {
		return 0
	}
	return len(q.q[len(q.q)-1].Raw)
}
