package sftp

import (
	"encoding"
	"sort"
)

// instead of focusing on the ring/structure
// focus on the role/service...
//
// packets come in and are split out to a pool of workers
// when finished those workers need to send their  responsePackets to
// another service/worker that gathers up responsePackets and makes
// sure they go out in the correct order.

// So it is an opaque service all the outgoing packets get sent to.
// All incoming ids get sent to it as well.

// So it is a service listening on 2 channels, uses the ordering information
// from the id-channel to order the packets coming  from the other channel.

// How does it store ordering information?

// Assuming ids are numericly ordered and that the lowest possible
// id will always be present in the set...
// We can then keep both the incoming ids and the packets in a sorted list.
// Both lists will be sorted by id (so in correct order).

// NOTE: Do this!
// --------------------------------------------------------------------
// Process with 2 branch select, listening to each channel.
// 0) start of loop

// Branch A
// 1) Wait for ids to come in and add them to id list.

// Branch B
// 1) Wait for a packet comes in.
// 2) Add it to the packet list.
// 3) The heads of each list are then compared and if they have the same ids
//    the packet is sent out and the entries removed.
// 4) Goto step 2 Until the lists are emptied or the ids don't match.
// 5) Goto step 0.
// --------------------------------------------------------------------

type packetSender interface {
	sendPacket(encoding.BinaryMarshaler) error
}

type packetSync struct {
	ids      chan uint32
	packets  chan responsePacket
	fini     chan struct{}
	incoming []uint32
	outgoing []responsePacket // need named type to sort
	sender   packetSender
}

// register incoming packets to be handled
// send id of 0 for packets without id
func (s packetSync) incomingPacketId(id uint32) {
	debug("incomingPacketId: %v", id)
	s.ids <- id
}

// register outgoing packets as being ready
func (s packetSync) readyPacket(pkt responsePacket) {
	s.packets <- pkt
}

// shut down packetSync worker
func (s packetSync) close() {
	s.fini <- struct{}{}
}

// process packets
func (s *packetSync) worker() {
	for {
		select {
		case id := <-s.ids:
			debug("incoming id: %v", id)
			s.incoming = append(s.incoming, id)
			sort.Slice(s.incoming, func(i, j int) bool {
				return s.incoming[i] < s.incoming[j]
			})
		case pkt := <-s.packets:
			debug("outgoing pkt: %v", pkt.id())
			s.outgoing = append(s.outgoing, pkt)
			sort.Slice(s.outgoing, func(i, j int) bool {
				return s.outgoing[i].id() < s.outgoing[j].id()
			})
		case <-s.fini:
			return
		}
		s.maybeSendPackets()
	}
}

// send as many packets as are ready
func (s *packetSync) maybeSendPackets() {
	for {
		if len(s.outgoing) == 0 || len(s.incoming) == 0 {
			debug("break! -- outgoing: %v; incoming: %v",
				len(s.outgoing), len(s.incoming))
			break
		}
		out := s.outgoing[0]
		in := s.incoming[0]
		debug("comparing packet ids: %v %v", in, out.id())
		debug("outgoing: %v", outfilter(s.outgoing))
		debug("incoming: %v", s.incoming)
		if in == out.id() {
			s.sender.sendPacket(out)
			// pop off heads
			copy(s.incoming, s.incoming[1:])            // shift left
			s.incoming = s.incoming[:len(s.incoming)-1] // remove last
			copy(s.outgoing, s.outgoing[1:])            // shift left
			s.outgoing = s.outgoing[:len(s.outgoing)-1] // remove last
		} else {
			break
		}
	}
}

func outfilter(o []responsePacket) []uint32 {
	res := make([]uint32, 0, len(o))
	for _, v := range o {
		res = append(res, v.id())
	}
	return res
}

func newpSync(sender packetSender) packetSync {
	s := packetSync{
		ids:      make(chan uint32, sftpServerWorkerCount),
		packets:  make(chan responsePacket, sftpServerWorkerCount),
		fini:     make(chan struct{}),
		incoming: make([]uint32, 0, sftpServerWorkerCount),
		outgoing: make([]responsePacket, 0, sftpServerWorkerCount),
		sender:   sender,
	}
	go s.worker()
	return s
}
