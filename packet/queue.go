package packet

import "github.com/pion/rtp"

type Queue struct {
	q []*rtp.Packet
}

func (q *Queue) Push(p *rtp.Packet) {
	q.q = append(q.q, p)
}

func (q *Queue) Pop() *rtp.Packet {
	p := q.q[0]
	q.q = q.q[1:]
	return p
}

func (q *Queue) Clear() {
	q.q = []*rtp.Packet{}
}

func (q *Queue) SizeOfNextRTP() int {
	return len(q.q[0].Raw)
}

func (q *Queue) SeqNrOfNextRTP() int {
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
	panic("implement me")
}

func (q *Queue) GetSizeOfLastFrame() int {
	panic("implement me")
}
