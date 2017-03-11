package sftp

import (
	"encoding"
	"testing"

	"github.com/stretchr/testify/assert"
)

type _sender struct {
	sent chan encoding.BinaryMarshaler
}

func newsender() *_sender {
	return &_sender{make(chan encoding.BinaryMarshaler)}
}

func (s _sender) sendPacket(p encoding.BinaryMarshaler) error {
	s.sent <- p
	return nil
}

type fakepacket uint32

func (fakepacket) MarshalBinary() ([]byte, error) {
	return []byte{}, nil
}

func (f fakepacket) id() uint32 {
	return uint32(f)
}

type pair struct {
	id  uint32
	pkt fakepacket
}

var ttable1 = []pair{
	pair{0, fakepacket(0)},
	pair{1, fakepacket(1)},
	pair{2, fakepacket(2)},
	pair{3, fakepacket(3)},
}

var ttable2 = []pair{
	pair{0, fakepacket(0)},
	pair{1, fakepacket(4)},
	pair{2, fakepacket(1)},
	pair{3, fakepacket(3)},
	pair{4, fakepacket(2)},
}

var tables = [][]pair{ttable1, ttable2}

func TestPacketSync(t *testing.T) {
	sender := newsender()
	s := newpSync(sender)
	for i := range tables {
		table := tables[i]
		for _, p := range table {
			s.incomingPacketId(p.id)
		}
		for _, p := range table {
			s.readyPacket(p.pkt)
		}
		count := 0
		for p := range sender.sent {
			id := p.(fakepacket).id()
			assert.Equal(t, int(id), count)
			count++
			if count == len(table) {
				break
			}
		}
	}
	s.close()
}
