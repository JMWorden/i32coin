package p2p

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"golang.org/x/exp/rand"

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
	"gonum.org/v1/gonum/stat/sampleuv"
)

const proto protocol.ID = "/p2p/1.0.0"
const internalBufSize int = 32 // size of buffer for internal channel
const peerBufSize int = 16     // size of buffer for peer channel
const toPeerOutSize int = 16   // size of buffer for copyied messages
const gossipSize int = 2       // number of peers to gossip messages to
const goalNumPeers int = 4     // goal number of peers

var gossipNdxs []int // buffer for peer selection during gossip
var server *TCPServer

// TCPServer is the interface to other nodes in the network
type TCPServer struct {
	port       int
	p2pHost    host.Host
	addr       string // address of this host
	adminIn    <-chan messages.LocalMsg
	adminOut   chan<- messages.LocalMsg
	internal   chan *Msg                  // channel to receive messages from peer connections
	peers      map[interface{}]*peerConn  // peer connections, indexed by peer address
	seenPeers  map[interface{}]struct{}   // seen peer addresses
	targets    []interface{}              // slice of targets, for effecient sampling
	toPeerOut  []*Msg                     // buffer of messages to be sent to peers
	peerOutNdx *int                       // increments everytime a new message to output is generated
	bcHeight   uint64                     // blockchain height
	roots      map[uint64]blockchain.Hash // merkle roots of blockchain (without genesis)
	randSrc    rand.Source
	pending    int // opened connections that still have an unknown id
}

// Init initializes TCPServer, registering structures with gob
func Init(port int, in <-chan messages.LocalMsg, out chan<- messages.LocalMsg) {
	//golog.SetAllLoggers(golog.LevelDebug)
	gob.Register(blockchain.Block{})
	gob.Register(helloData{})
	gob.Register(peerData{})
	server = newTCPServer(port, in, out)
	gossipNdxs = make([]int, gossipSize)
	server.start()
}

func newTCPServer(port int, in <-chan messages.LocalMsg, out chan<- messages.LocalMsg) *TCPServer {
	s := TCPServer{port: port, adminIn: in, adminOut: out}
	s.internal = make(chan *Msg, internalBufSize)
	s.peers = make(map[interface{}]*peerConn)
	s.seenPeers = make(map[interface{}]struct{})
	s.toPeerOut = make([]*Msg, toPeerOutSize)
	peerOutNdx := 0
	s.peerOutNdx = &peerOutNdx
	s.targets = make([]interface{}, 0, goalNumPeers)
	s.randSrc = rand.New(rand.NewSource(uint64(time.Now().UnixNano())))
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
	// generate key pair for host
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, s.randSrc.(io.Reader))
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
	s.addr = addr.Encapsulate(hostAddr).String()
	log.Printf("this host ip: %s\n", s.addr)
	log.Printf("Now run 'go run main.go -port %d -peer %s'\n", s.port+1, s.addr)

	s.p2pHost = host

	return nil
}

// HostAddr returns the server's address
func HostAddr() string {
	return server.addr
}

// Genesis should be called for the first peering node. Awaits connection to peer
func Genesis() {
	server.listen()
	server.managePeers()
}

// Peer connects to peer and listenss for data
func Peer(target string) {
	server.listen()
	server.seenPeers[target] = struct{}{}
	server.dial(target)
	server.managePeers()
}

func (s *TCPServer) listen() {
	log.Println("accept peering requests")
	s.p2pHost.SetStreamHandler(proto, handleStream)
}

func (s *TCPServer) dial(target string) {
	log.Println("dialing ", target)
	peerid, targetAddr := extractPeer(target)
	s.p2pHost.Peerstore().AddAddr(peerid, targetAddr, peerstore.PermanentAddrTTL)

	log.Println("opening stream w/ ", target)
	ns, err := s.p2pHost.NewStream(context.Background(), peerid, proto)
	if err != nil {
		log.Println("error: could not open stream w/ peer, ", err)
	} else {
		conn := s.launchNewPeer(ns, target)
		s.internal <- &Msg{Mtype: initConn, conn: conn}
	}
}

// extracts target's peer ID from the given multiaddress
func extractPeer(target string) (peer.ID, multiaddr.Multiaddr) {
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
		log.Fatalln("fatal: could not get peer addr, ", err)
	}

	targetAddr := ipfsAddr.Decapsulate(peerAddr)

	return peerid, targetAddr
}

// only called when stream connects, and starts a stream with this protocol
func handleStream(ns network.Stream) {
	log.Println("received new stream")
	server.launchNewPeer(ns, "")
}

func (s *TCPServer) launchNewPeer(ns network.Stream, target string) *peerConn {
	in := make(chan *Msg, peerBufSize)
	conn := newPeerConn(ns, in, s.internal, target)
	conn.rwStream(ns)
	log.Println("p2p server: launched peer ", target)
	return conn
}

func (s *TCPServer) initHandshake(conn *peerConn) {
	log.Println("p2p server: starting hello exchange with ", conn.target)
	helloMsg := s.makeHello()
	select {
	case conn.in <- helloMsg:
	default:
	}
}

func (s *TCPServer) managePeers() {
	log.Println("managing peers")

	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	decoder := gob.NewDecoder(buf)

	// peer awaiting range
	var awaitingRange string

	for {
		select {
		case msg := <-s.adminIn:
			log.Printf("p2p server: handling admin message")
			p2pmsg := Msg{}
			switch msg.Mtype {
			case messages.ShareBlock:
				b := msg.Block.(*blockchain.Block)
				_, seen := s.roots[b.Height]
				if seen {
					break // skip if already seen
				}
				s.bcHeight = b.Height
				s.roots[s.bcHeight] = b.MerkleRoot
				p2pmsg.Mtype = block
				p2pmsg.Payload = msg.Block
				cpy, err := s.bufferMsg(&p2pmsg, encoder, decoder)
				if err == nil {
					s.gossip(cpy)
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
				_, found := s.peers[awaitingRange]
				if found {
					sendRange(s.peers[awaitingRange].in, msg.Block.([]*blockchain.Block))
				}
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
				s.removePeer(msg.conn)
				break
			case hello:
				log.Println("p2p server: processing hello from ", msg.conn.target)
				if s.registerPeer(msg.conn) {
					resp := s.makeHello()
					resp.Mtype = helloRes
					s.direct(msg.conn, resp)
					s.sharePeers(s.targets, msg.conn)
					s.handleHellos(resp, msg)
				}
				break
			case helloRes:
				log.Println("p2p server: processing hello response from ", msg.conn.target)
				if s.registerPeer(msg.conn) {
					s.handleHellos(s.makeHello(), msg)
				}
				break
			case rangeReq:
				awaitingRange = msg.conn.target
				s.adminOut <- messages.LocalMsg{Mtype: messages.RangeReq, Height: msg.Payload.(uint64)}
				break
			case peers:
				s.handlePeers(msg)
				break
			case initConn:
				s.initHandshake(msg.conn)
				break
			}
			break
		}
	}
}

// MapVals returns slice of values in map
func MapVals(m map[interface{}]interface{}) []interface{} {
	vals := make([]interface{}, len(m))

	ndx := 0
	for _, val := range m {
		vals[ndx] = val
		ndx++
	}

	return vals
}

// RemoveOne removes first instance of target from slice. Does not preserve order.
// Returns new slice
func RemoveOne(target interface{}, src []interface{}) []interface{} {
	tndx := -1
	for ndx, val := range src {
		if val == target {
			tndx = ndx
			break
		}
	}

	if tndx > -1 {
		src[tndx] = src[len(src)-1]
		src = src[:len(src)-1]
	}

	return src
}

func (s *TCPServer) removePeer(conn *peerConn) {
	log.Println("p2p server: removing peer ", conn.target)

	select {
	case _, ok := <-conn.in:
		if !ok {
			break
		}
	default:
		close(conn.in)
	}
	if conn.target != "" {
		delete(s.peers, conn.target)
		s.targets = RemoveOne(conn.target, s.targets)
		delete(s.seenPeers, conn.target)
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
		case p.in <- msg:
		default:
			break
		}
	}
}

// Non-blocking send message to random subset of peer connections
func (s *TCPServer) gossip(msg *Msg) {
	if len(s.peers) <= gossipSize {
		s.broadcast(msg)
	} else {
		log.Printf("p2p server: gossiping %s message to %d peers\n", msg.Mtype, gossipSize)
		sampleuv.WithoutReplacement(gossipNdxs, len(s.peers), s.randSrc)
		for ndx := range gossipNdxs {
			select {
			case s.peers[s.targets[ndx].(string)].in <- msg:
			default:
				break
			}
		}
	}
}

// Non-blocking send message to a specific peer
func (s *TCPServer) direct(to *peerConn, msg *Msg) {
	select {
	case to.in <- msg:
	default:
		break
	}
}

func (s *TCPServer) makeHello() *Msg {
	msg := Msg{Mtype: hello}
	data := helloData{Addr: s.addr}

	data.Roots = make([]blockchain.Hash, len(s.roots)+1)
	for h := uint64(1); h <= s.bcHeight; h++ {
		data.Roots[h] = s.roots[h]
	}

	msg.Height = s.bcHeight
	msg.Payload = data
	return &msg
}

// Registers peer address, returns true on success
func (s *TCPServer) registerPeer(conn *peerConn) bool {
	peerAddr := conn.target

	// Abort registration if connection already exists
	_, duplicate := s.peers[peerAddr]
	if duplicate {
		log.Printf("p2p server: peer already registered, %s", peerAddr)
		conn.target = ""
		if strings.Compare(peerAddr, s.addr) > 0 {
			log.Println("skipping ", peerAddr)
		} else {
			log.Println("p2p server: closing ", peerAddr)
			s.removePeer(conn)
		}
		return false
	}

	log.Println("p2p server: registering peer ", peerAddr)
	s.seenPeers[conn.target] = struct{}{}
	s.peers[peerAddr] = conn
	s.targets = append(s.targets, peerAddr)
	return true
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
		case s.peers[resp.conn.target].in <- &Msg{Mtype: rangeReq, Height: s.bcHeight, Payload: h + 1}:
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

func (s *TCPServer) sharePeers(targets []interface{}, mustSend *peerConn) {
	log.Printf("p2p server: sharing %d peers\n", len(targets))
	msg := Msg{Mtype: peers, Payload: peerData{Addrs: targets}}
	s.gossip(&msg)
	if mustSend != nil {
		s.direct(mustSend, &msg)
	}
}

func (s *TCPServer) handlePeers(msg *Msg) {
	remotePeers := msg.Payload.(peerData).Addrs
	log.Printf("p2p server: have %d peers, examining %d more\n", len(s.peers), len(remotePeers))
	targets := make([]interface{}, 0, len(remotePeers))

	for _, rp := range remotePeers {
		_, seen := s.seenPeers[rp]
		if !seen && rp != s.addr {
			targets = append(targets, rp)
			log.Println("p2p server: adding target: ", rp)
			s.seenPeers[rp] = struct{}{}
		}
	}

	slots := goalNumPeers - len(s.peers)
	if len(targets) > 0 {
		rand.Shuffle(len(targets), func(i, j int) { targets[i], targets[j] = targets[j], targets[i] })
		for ndx, t := range targets {
			log.Println("p2p server: selected target: ", t)
			if ndx-1 < slots {
				go s.dial(t.(string))
			}
		}
		log.Println("p2p server: append produced length ", len(s.targets)+len(targets))
		fullTargets := make([]interface{}, 0, len(s.targets)+len(targets))
		fullTargets = append(fullTargets, s.targets...)
		fullTargets = append(fullTargets, targets...)
		s.sharePeers(fullTargets, nil)
	}
}
