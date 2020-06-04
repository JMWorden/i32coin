package p2p

import (
	"bufio"
	"encoding/gob"
	"log"

	"github.com/JMWorden/int32coin/blockchain"
	"github.com/libp2p/go-libp2p-core/network"
)

type mType int

const (
	candidate mType = iota
	block
	hello
	helloRes
	removeMe
	initConn
	rangeReq
)

func (t mType) String() string {
	switch t {
	case candidate:
		return "candidate"
	case block:
		return "block"
	case hello:
		return "hello"
	case helloRes:
		return "hello-response"
	case initConn:
		return "init-connection"
	case rangeReq:
		return "range-request"
	default:
		return "undefined"
	}
}

// Msg is message send between peers
type Msg struct {
	Mtype   mType // message type
	Height  uint64
	PeerNdx int
	Payload interface{}
}

type helloData struct {
	Roots []blockchain.Hash
}

func (m *Msg) send(encoder *gob.Encoder) error {
	err := encoder.Encode(m)
	if err != nil {
		log.Println("p2pMsg enocde error: ", err)
	}
	return err
}

func recv(decoder *gob.Decoder) (*Msg, error) {
	m := Msg{}

	err := decoder.Decode(&m)
	if err != nil {
		log.Println("p2pMsg decode error: ", err)
	}
	return &m, err
}

type peerConn struct {
	ndx int
	ns  network.Stream
	in  <-chan *Msg
	out chan<- *Msg
}

func newPeerConn(ndx int, ns network.Stream, in <-chan *Msg, out chan<- *Msg) *peerConn {
	peer := peerConn{ndx: ndx, ns: ns, in: in, out: out}
	return &peer
}

func (p *peerConn) rwStream(ns network.Stream) {
	rw := bufio.NewReadWriter(bufio.NewReader(ns), bufio.NewWriter(ns))

	go p.peerIn(rw)
	go p.peerOut(rw)
}

func (p *peerConn) peerIn(rw *bufio.ReadWriter) {
	decoder := gob.NewDecoder(rw)
	for {
		msg, err := recv(decoder)
		if err == nil {
			log.Println("received p2p message ", msg.Mtype)
			msg.PeerNdx = p.ndx
			p.out <- msg
		} else {
			log.Println("error: failed to receive p2p message, ", err)
			p.out <- &Msg{Mtype: removeMe, PeerNdx: p.ndx}
			log.Println("reader closing connection to peer")
			return
		}
	}
}

func (p *peerConn) peerOut(rw *bufio.ReadWriter) {
	encoder := gob.NewEncoder(rw)

	for msg := range p.in {
		err := msg.send(encoder)
		if err == nil {
			log.Println("peer: buffered p2p message send", msg.Mtype)
			rw.Flush()
			log.Println("peer: flushed p2p message send buf", msg.Mtype)
		} else {
			log.Println("peer error: failed to send p2p message, ", err)
			p.out <- &Msg{Mtype: removeMe, PeerNdx: p.ndx}
			log.Println("peer: closing connection to peer")
			return
		}
	}

	log.Println("peer: writer exiting")
}
