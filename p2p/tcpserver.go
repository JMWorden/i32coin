package p2p

import (
	"bufio"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"math/rand"
	"time"

	"github.com/JMWorden/int32coin/blockchain"
	"github.com/JMWorden/int32coin/messages"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/multiformats/go-multiaddr"
)

const proto protocol.ID = "/p2p/1.0.0"

type p2pType int

const (
	candidate p2pType = iota
	block
)

type p2pMsg struct {
	mtype   p2pType // message type
	payload interface{}
}

func (m *p2pMsg) send(encoder *gob.Encoder) error {
	err := encoder.Encode(m)
	if err != nil {
		log.Println("p2pMsg enocde error: ", err)
	}
	return err
}

func recv(decoder *gob.Decoder) (*p2pMsg, error) {
	m := p2pMsg{}

	err := decoder.Decode(&m)
	if err != nil {
		log.Println("p2pMsg decode error: ", err)
	}
	return &m, err
}

// TCPServer is the interface to other nodes in the network
type TCPServer struct {
	port     int
	p2pHost  host.Host
	adminIn  <-chan messages.LocalMsg
	adminOut chan<- messages.LocalMsg
}

// Init initializes TCPServer, registering structures with gob
func Init() {
	gob.Register(blockchain.Block{})
}

// NewTCPServer creates a new TCPServer with the given port number
func NewTCPServer(port int, in <-chan messages.LocalMsg, out chan<- messages.LocalMsg) *TCPServer {
	s := TCPServer{port: port, adminIn: in, adminOut: out}
	return &s
}

// Start starts TCPServer, creating and starting a host and accepting incomming peers
func (s *TCPServer) Start() {
	err := s.startHost()
	if err != nil {
		log.Fatalln("could not start host")
	}
}

func (s *TCPServer) startHost() error {
	var rdr io.Reader = rand.New(rand.NewSource(time.Now().UnixNano()))

	// generate key pair for host
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rdr)
	if err != nil {
		log.Println("could not generate private key")
		return err
	}

	listenAddrStr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", s.port)
	opts := []libp2p.Option{libp2p.ListenAddrStrings(listenAddrStr),
		libp2p.Identity(priv)}

	host, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		log.Println("could not create p2p host")
		return err
	}

	hostAddrStr := fmt.Sprintf("/ipfs/%s", host.ID().Pretty())
	hostAddr, _ := multiaddr.NewMultiaddr(hostAddrStr)

	addr := host.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)
	log.Printf("this host ip: %s\n", fullAddr)
	log.Printf("Now run 'go run main.go -port %d -peer %s'\n", s.port+1, fullAddr)

	s.p2pHost = host

	return nil
}

// Genesis should be called for the first peering node. Awaits connection to peer
func (s *TCPServer) Genesis() {
	s.acceptPeers()
}

// Peer connects to peer and listenss for data
func (s *TCPServer) Peer(target string) {
	s.acceptPeers()
	s.connectPeer(target)
}

func (s *TCPServer) acceptPeers() {
	log.Println("accept peering requests")
	s.p2pHost.SetStreamHandler(proto, s.handleStream)
}

func (s *TCPServer) connectPeer(target string) {
	peerid, targetAddr := s.extractPeer(target)
	s.p2pHost.Peerstore().AddAddr(peerid, targetAddr, peerstore.PermanentAddrTTL)

	log.Println("opening stream w/ peer")
	ns, err := s.p2pHost.NewStream(context.Background(), peerid, proto)
	if err != nil {
		log.Fatalln("fatal: could not open stream w/ peer, ", err)
	}
	s.rwStream(ns)
}

// extracts target's peer ID from the given multiaddress
func (s *TCPServer) extractPeer(target string) (peer.ID, multiaddr.Multiaddr) {
	ipfsAddr, err := multiaddr.NewMultiaddr(target)
	if err != nil {
		log.Fatalln("fatal: could not get peer ipfs addr, ", err)
	}

	pproto, err := ipfsAddr.ValueForProtocol(multiaddr.P_IPFS)
	if err != nil {
		log.Fatalln("fatal: could not get protocol, ", err)
	}

	peerid, err := peer.IDB58Decode(pproto)
	if err != nil {
		log.Fatalln("fatal: could not get peer id, ", err)
	}

	peerAddrStr := fmt.Sprintf("/ipfs/%s", peer.IDB58Encode(peerid))
	peerAddr, _ := multiaddr.NewMultiaddr(peerAddrStr)
	targetAddr := ipfsAddr.Decapsulate(peerAddr)

	return peerid, targetAddr
}

// Only called when stream connects, and starts a stream with this protocol
func (s *TCPServer) handleStream(ns network.Stream) {
	log.Println("received new stream")
	s.rwStream(ns)
}

func (s *TCPServer) rwStream(ns network.Stream) {
	rw := bufio.NewReadWriter(bufio.NewReader(ns), bufio.NewWriter(ns))

	go s.peerIn(rw)
	go s.peerOut(rw)
}

func (s *TCPServer) peerIn(rw *bufio.ReadWriter) {
	decoder := gob.NewDecoder(rw)
	for {
		p2pmsg, err := recv(decoder)
		if err == nil {
			log.Println("received p2p message")
			switch p2pmsg.mtype {
			case candidate:
				break
			}
		} else {
			log.Println("error: failed to receive p2p message, ", err)
		}
	}
}

func (s *TCPServer) peerOut(rw *bufio.ReadWriter) {
	encoder := gob.NewEncoder(rw)

	for msg := range s.adminIn {
		p2pmsg := p2pMsg{}
		switch msg.Mtype {
		case messages.ShareBlock:
			p2pmsg.mtype = candidate
			p2pmsg.payload = msg.Block
			break
		case messages.CandidateBlock:
			p2pmsg.mtype = block
			p2pmsg.payload = msg.Block
			break
		}
		err := p2pmsg.send(encoder)
		if err == nil {
			log.Println("sent p2p message")
		} else {
			log.Println("error: failed to send p2p message, ", err)
		}
	}
}
