package p2p

import (
	"bufio"
	"encoding/gob"
	"log"
	"time"

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
	peers
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
	case removeMe:
		return "remove-me"
	case peers:
		return "peers"
	default:
		return "undefined"
	}
}

// Msg is message send between peers
type Msg struct {
	Mtype   mType // message type
	Height  uint64
	Payload interface{}
	conn    *peerConn
}

type helloData struct {
	Roots []blockchain.Hash
	Addr  string
}

type peerData struct {
	Addrs []interface{} // peer addresses
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
	target string
	in     chan *Msg
	out    chan<- *Msg
	since  int64
}

func newPeerConn(ns network.Stream, in chan *Msg, out chan<- *Msg, target string) *peerConn {
	peer := peerConn{in: in, out: out, target: target}
	peer.since = time.Now().UnixNano()
	return &peer
}

// Returns nanoseconds since creation
func (p *peerConn) duration() int64 {
	return time.Now().UnixNano() - p.since
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
			log.Println("peer: got msg ", msg.Mtype, "from ", p.target)
			msg.conn = p
			if msg.Mtype == hello {
				p.target = msg.Payload.(helloData).Addr
			}
			p.out <- msg
		} else {
			log.Printf("error: failed to receive %s message, %s", msg.Mtype, err)
			p.out <- &Msg{Mtype: removeMe, conn: p}
			log.Println("reader closing connection to peer")
			return
		}
	}
}

func (p *peerConn) peerOut(rw *bufio.ReadWriter) {
	encoder := gob.NewEncoder(rw)

	for msg := range p.in {
		msg.conn = nil
		err := msg.send(encoder)
		if err == nil {
			rw.Flush()
			log.Println("peer: sent msg ", msg.Mtype, "to ", p.target)
		} else {
			log.Printf("error: failed to send %s message, %s", msg.Mtype, err)
			p.out <- &Msg{Mtype: removeMe, conn: p}
			log.Println("peer: closing connection to peer")
			return
		}
	}

	log.Println("peer: writer exiting")
}
