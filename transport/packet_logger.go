package transport

import (
	"log"

	"github.com/pion/rtp"
)

type Logger struct{}

func (p *Logger) Write(b []byte) (int, error) {
	packet := &rtp.Packet{}
	err := packet.Unmarshal(b)
	if err != nil {
		return 0, err
	}
	log.Println(packet)
	return len(b), nil
}
