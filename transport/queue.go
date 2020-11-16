package transport

import (
	"github.com/pion/rtp"
)

type RTPQueueItem struct {
	Packet    *rtp.Packet
	Timestamp uint32
}

type Queue struct {
	q []*RTPQueueItem
}

func NewQueue() *Queue {
	return &Queue{
		make([]*RTPQueueItem, 0),
	}
}

func (q *Queue) Push(p *RTPQueueItem) {
	q.q = append(q.q, p)
}

func (q *Queue) Pop() *RTPQueueItem {
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
	q.q = []*RTPQueueItem{}
}

func (q *Queue) SizeOfNextRTP() int {
	if len(q.q) <= 0 {
		return 0
	}
	return len(q.q[0].Packet.Raw)
}

func (q *Queue) SeqNrOfNextRTP() int {
	if len(q.q) <= 0 {
		return 0
	}
	return int(q.q[0].Packet.SequenceNumber)
}

func (q *Queue) BytesInQueue() int {
	size := 0
	for _, p := range q.q {
		size += len(p.Packet.Raw)
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
	d := f - float64(q.q[0].Timestamp)
	return d
}

func (q *Queue) GetSizeOfLastFrame() int {
	if len(q.q) <= 0 {
		return 0
	}
	return len(q.q[len(q.q)-1].Packet.Raw)
}
