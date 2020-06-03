package p2p

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"time"

	"github.com/JMWorden/int32coin/messages"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/multiformats/go-multiaddr"
)

const protocol string = "/p2p/1.0.0"

// TCPServer is the interface to other nodes in the network
type TCPServer struct {
	port    int
	target  string // ip of target peer, "" if none
	p2pHost host.Host
}

// NewTCPServer creates a new TCPServer with the given port number
func NewTCPServer(port int, target string) *TCPServer {
	s := TCPServer{port: port}
	s.target = target
	return &s
}

// Start starts TCPServer, creating a tcp host and listening for data to send or recieve
func (s *TCPServer) Start(in <-chan messages.LocalMsg, out chan<- messages.LocalMsg) error {
	p2pHost, err := startHost(s.port)
	if err != nil {
		return err
	}
	s.p2pHost = p2pHost

	if s.target == "" {
		s.listenForConnect()
	}

	for msg := range in {
		switch msg.Mtype {
		case messages.ShareBlock:
			break
		case messages.CandidateBlock:
			break
		}
	}

	return nil
}

func (s *TCPServer) listenForConnect() {
	log.Println("listening for connections")

	s.p2pHost.SetStreamHandler(protocol, handleStream)
}

func startHost(port int) (host.Host, error) {
	var rdr io.Reader = rand.New(rand.NewSource(time.Now().UnixNano()))

	// generate key pair for host
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rdr)
	if err != nil {
		log.Println("could not generate private key")
		return nil, err
	}

	listenAddrStr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port)
	opts := []libp2p.Option{libp2p.ListenAddrStrings(listenAddrStr),
		libp2p.Identity(priv)}

	host, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		log.Println("could not create p2p host")
		return nil, err
	}

	hostAddrStr := fmt.Sprintf("/ipfs/%s", host.ID().Pretty())
	hostAddr, _ := multiaddr.NewMultiaddr(hostAddrStr)

	addr := host.Addrs()[0]
	fullAddr := addr.Encapsulate(hostAddr)
	log.Printf("this host ip: %s\n", fullAddr)
	log.Printf("Now run 'go run main.go -l %d -d %s'\n", port+1, fullAddr)

	return host, nil
}

func handleStream(s network.Strean) {
	log.Println("recieved new stream")

	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))

}
