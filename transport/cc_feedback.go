package transport

import (
	"encoding/binary"
	"fmt"

	"github.com/pion/rtcp"
)

type CCFeedback struct {
	Header          *rtcp.Header
	SenderSSRC      uint32
	Reports         []*SSRCReport
	ReportTimestamp uint32
}

func (c *CCFeedback) String() string {
	s := "FEEDBACK PACKET:\n"
	s += fmt.Sprintf("Header: %v\n", c.Header)
	s += fmt.Sprintf("Reports: %v\n", len(c.Reports))
	for _, r := range c.Reports {
		//s += "{\n"
		s += fmt.Sprintf("%v-%v\n", r.BeginSeq, r.NumReports)
		//s += "}\n"
	}
	s += fmt.Sprintf("Timestamp: %v\n", c.ReportTimestamp)
	return s
}

func (c *CCFeedback) UnmarshalBinary(data []byte) error {
	c.Header = &rtcp.Header{}
	err := c.Header.Unmarshal(data[:4])
	if err != nil {
		return err
	}

	c.SenderSSRC = binary.BigEndian.Uint32(data[4:8])
	c.ReportTimestamp = binary.BigEndian.Uint32(data[len(data)-4:])

	for i := 8; i+4 < len(data); {
		r := &SSRCReport{}
		err := r.UnmarshalBinary(data[i:])
		if err != nil {
			return err
		}
		i += 8 + int(r.NumReports)*2
		if r.NumReports%2 != 0 {
			i += 2
		}
		c.Reports = append(c.Reports, r)
	}
	return nil
}

type SSRCReport struct {
	StreamSSRC uint32
	BeginSeq   uint16
	NumReports uint16
	Reports    []*StreamReport
}

func (s *SSRCReport) String() string {
	r := fmt.Sprintf("StreamSSRC: %v\n", s.StreamSSRC)
	r += fmt.Sprintf("Begin Seq: %v\n", s.BeginSeq)
	r += fmt.Sprintf("Num Reports: %v\n", s.NumReports)
	r += fmt.Sprintf("Reports: %v\n", s.Reports)
	return r
}

func (s *SSRCReport) UnmarshalBinary(data []byte) error {
	s.StreamSSRC = binary.BigEndian.Uint32(data[0:4])
	s.BeginSeq = binary.BigEndian.Uint16(data[4:6])
	s.NumReports = binary.BigEndian.Uint16(data[6:8])
	for i := 0; i <= int(s.NumReports); i++ {
		r := &StreamReport{}
		n := 2*i + 8
		err := r.UnmarshalBinary(data[n : n+2])
		if err != nil {
			return err
		}
		s.Reports = append(s.Reports, r)
	}
	return nil
}

type StreamReport struct {
	L                 bool
	ECN               byte
	ArrivalTimeOffset uint16
}

func (s *StreamReport) UnmarshalBinary(data []byte) error {
	v := binary.BigEndian.Uint16(data)
	s.L = data[0]&0x80 == 0x80
	s.ECN = data[0] & 0x60 >> 5
	s.ArrivalTimeOffset = v & 0x1FFF
	return nil
}
