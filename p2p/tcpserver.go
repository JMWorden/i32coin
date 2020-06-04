package p2p

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"math/rand"
	"time"

	"github.com/JMWorden/int32coin/blockchain"
	"github.com/JMWorden/int32coin/messages"

	//golog "github.com/ipfs/go-log"
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
const internalBufSize int = 32 // size of buffer for internal channel
const peerBufSize int = 16     // size of buffer for peer channel
const toPeerOutSize int = 16   // size of buffer for copyied messages

var server *TCPServer

// TCPServer is the interface to other nodes in the network
type TCPServer struct {
	port       int
	p2pHost    host.Host
	adminIn    <-chan messages.LocalMsg
	adminOut   chan<- messages.LocalMsg
	internal   chan *Msg                  // channel to receive messages from peer connections
	peers      map[int]chan<- *Msg        // channels to send messages to peer connections
	toPeerOut  []*Msg                     // buffer of messages to be sent to peers
	peerNdx    int                        // increments everytime a peer connection is made
	peerOutNdx *int                       // increments everytime a new message to output is generated
	bcHeight   uint64                     // blockchain height
	roots      map[uint64]blockchain.Hash // merkle roots of blockchain (without genesis)
}

// Init initializes TCPServer, registering structures with gob
func Init(port int, in <-chan messages.LocalMsg, out chan<- messages.LocalMsg) {
	//golog.SetAllLoggers(golog.LevelDebug)
	gob.Register(blockchain.Block{})
	gob.Register(helloData{})
	server = newTCPServer(port, in, out)
	server.start()
}

func newTCPServer(port int, in <-chan messages.LocalMsg, out chan<- messages.LocalMsg) *TCPServer {
	s := TCPServer{port: port, adminIn: in, adminOut: out}
	s.internal = make(chan *Msg, internalBufSize)
	s.peers = make(map[int]chan<- *Msg)
	s.toPeerOut = make([]*Msg, toPeerOutSize)
	peerOutNdx := 0
	s.peerOutNdx = &peerOutNdx
	s.roots = make(map[uint64]blockchain.Hash)
	return &s
}

// starts host
func (s *TCPServer) start() {
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
func Genesis() {
	server.listen()
	server.managePeers()
}

// Peer connects to peer and listenss for data
func Peer(target string) {
	server.listen()
	server.dial(target)
	server.managePeers()
}

func (s *TCPServer) listen() {
	log.Println("accept peering requests")
	s.p2pHost.SetStreamHandler(proto, handleStream)
}

func (s *TCPServer) dial(target string) {
	peerid, targetAddr := s.extractPeer(target)
	s.p2pHost.Peerstore().AddAddr(peerid, targetAddr, peerstore.PermanentAddrTTL)

	log.Println("opening stream w/ peer")
	ns, err := s.p2pHost.NewStream(context.Background(), peerid, proto)
	if err != nil {
		log.Fatalln("fatal: could not open stream w/ peer, ", err)
	}
	s.launchNewPeer(ns)
	s.internal <- &Msg{Mtype: initConn, PeerNdx: s.peerNdx - 1}
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
	peerAddr, err := multiaddr.NewMultiaddr(peerAddrStr)
	if err != nil {
		log.Fatalln("fatal: could not get peer addr")
	}

	targetAddr := ipfsAddr.Decapsulate(peerAddr)

	return peerid, targetAddr
}

// only called when stream connects, and starts a stream with this protocol
func handleStream(ns network.Stream) {
	log.Println("received new stream")
	server.launchNewPeer(ns)
}

func (s *TCPServer) launchNewPeer(ns network.Stream) {
	in := make(chan *Msg, peerBufSize)
	s.peers[s.peerNdx] = in
	newPeerConn(s.peerNdx, ns, in, s.internal).rwStream(ns)
	log.Println("p2p server: launched peer ", s.peerNdx)
	s.peerNdx++
}

func (s *TCPServer) managePeers() {
	log.Println("managing peers")

	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	decoder := gob.NewDecoder(buf)

	// peer awaiting range
	var awaitingRange int

	for {
		select {
		case msg := <-s.adminIn:
			log.Printf("p2p server: handling admin message")
			p2pmsg := Msg{}
			switch msg.Mtype {
			case messages.ShareBlock:
				s.bcHeight = msg.Block.(*blockchain.Block).Height
				s.roots[s.bcHeight] = msg.Block.(*blockchain.Block).MerkleRoot
				p2pmsg.Mtype = block
				p2pmsg.Payload = msg.Block
				cpy, err := s.bufferMsg(&p2pmsg, encoder, decoder)
				if err == nil {
					s.broadcast(cpy)
				}
				break
			case messages.CandidateBlock:
				p2pmsg.Mtype = candidate
				p2pmsg.Payload = msg.Block
				cpy, err := s.bufferMsg(&p2pmsg, encoder, decoder)
				if err == nil {
					s.broadcast(cpy)
				}
				break
			case messages.Range:
				sendRange(s.peers[awaitingRange], msg.Block.([]*blockchain.Block))
				break
			}
			break
		case msg := <-s.internal:
			log.Printf("p2p server: handling internal message")
			switch msg.Mtype {
			case candidate:
				block := msg.Payload.(blockchain.Block)
				s.adminOut <- messages.LocalMsg{Mtype: messages.RemoteCandidate, Block: &block}
				break
			case block:
				block := msg.Payload.(blockchain.Block)
				s.adminOut <- messages.LocalMsg{Mtype: messages.AddBlock, Block: &block}
				break
			case removeMe:
				close(s.peers[msg.PeerNdx])
				delete(s.peers, msg.PeerNdx)
				break
			case initConn:
				log.Println("p2p server: starting hello exchange with ", msg.PeerNdx)
				helloMsg := s.makeHello(msg.PeerNdx)
				select {
				case s.peers[msg.PeerNdx] <- helloMsg:
				default:
				}
				break
			case hello:
				log.Println("p2p server: processing hello from ", msg.PeerNdx)
				resp := s.makeHello(msg.PeerNdx)
				resp.Mtype = helloRes
				select {
				case s.peers[msg.PeerNdx] <- resp:
				default:
				}
				s.handleHellos(resp, msg)
				break
			case helloRes:
				log.Println("p2p server: processing hello response from ", msg.PeerNdx)
				s.handleHellos(s.makeHello(msg.PeerNdx), msg)
				break
			case rangeReq:
				awaitingRange = msg.PeerNdx
				s.adminOut <- messages.LocalMsg{Mtype: messages.RangeReq, Height: msg.Payload.(uint64)}
				break
			}
			break
		}
	}
}

func (s *TCPServer) bufferMsg(msg *Msg, encoder *gob.Encoder, decoder *gob.Decoder) (*Msg, error) {
	ndx := *s.peerOutNdx % toPeerOutSize
	(*s.peerOutNdx)++

	err := msg.send(encoder)
	if err != nil {
		log.Println("error: failed to buffer message, ", err)
		return nil, err
	}

	cpy, err := recv(decoder)
	if err != nil {
		log.Println("error: failed to unbuffer messages, ", err)
		return nil, err
	}

	s.toPeerOut[ndx] = cpy

	return cpy, nil
}

// Non-blocking send message to all peer connections
func (s *TCPServer) broadcast(msg *Msg) {
	log.Printf("p2p server: broadcasting %s message to peers\n", msg.Mtype)
	for _, p := range s.peers {
		select {
		case p <- msg:
		default:
			break
		}
	}
}

func (s *TCPServer) makeHello(peerNdx int) *Msg {
	msg := Msg{Mtype: hello, PeerNdx: peerNdx}
	data := helloData{}

	data.Roots = make([]blockchain.Hash, len(s.roots)+1)
	for h := uint64(1); h <= s.bcHeight; h++ {
		data.Roots[h] = s.roots[h]
	}

	msg.Height = s.bcHeight
	msg.Payload = data
	return &msg
}

func (s *TCPServer) handleHellos(hello *Msg, resp *Msg) {
	if resp.Height > hello.Height {
		log.Println("p2p server: peer's hello has longer blockchain")

		remoteRoots := resp.Payload.(helloData).Roots
		localRoots := hello.Payload.(helloData).Roots

		// remove differing blocks at end of chain
		h := uint64(hello.Height)
		for h > 0 {
			if remoteRoots[h].Equals(localRoots[h]) {
				break
			}
			h--
		}

		// remove [h+1:end]
		for rh := h + 1; rh <= s.bcHeight; rh++ {
			delete(s.roots, rh)
		}
		s.bcHeight = h
		s.adminOut <- messages.LocalMsg{Mtype: messages.RemoveBlocks, Height: h + 1}

		// request [h+1:end]
		select {
		case s.peers[resp.PeerNdx] <- &Msg{Mtype: rangeReq, Height: s.bcHeight, Payload: h + 1}:
		default:
		}
	} else {
		log.Println("p2p server: peer's hello didn't have longer blockchain")
	}
}

func sendRange(peerChan chan<- *Msg, blocks []*blockchain.Block) {
	log.Println("p2p server: sending block range of size", len(blocks))

	for _, b := range blocks {
		log.Println("p2p server: sending block ", b.Height)
		msg := Msg{}
		msg.Mtype = block
		msg.Height = b.Height
		msg.Payload = b
		select {
		case peerChan <- &msg:
		default:
		}
	}
}
