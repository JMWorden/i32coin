package network

import "github.com/JMWorden/int32coin/messages"

type localMsg = messages.LocalMsg

const servBufSize int = 64

// Server routes administrative messages between local go routines
type Server struct {
	Serv      chan localMsg // admin in channel for modules
	WalAdmin  chan localMsg // admin out channel for wallet module
	MineAdmin chan localMsg // admin out channel for mine module
	BcAdmin   chan localMsg // admin out channel for blockchain module
	NetAdmin  chan localMsg // admin out channel for network interface
	Info      chan localMsg // debug channel
}

// NewServer returns a new server with initialized channels
func NewServer() *Server {
	s := Server{}
	s.Serv = make(chan localMsg, servBufSize)
	s.WalAdmin = make(chan localMsg)
	s.MineAdmin = make(chan localMsg)
	s.BcAdmin = make(chan localMsg)
	s.NetAdmin = make(chan localMsg)
	s.Info = make(chan localMsg)

	return &s
}

// Route routes messages between go routines
func (s *Server) Route() {
	for msg := range s.Serv {
		switch msg.Mtype {
		case messages.AddBlock:
			s.BcAdmin <- msg // send block from miner/network to blockchain
			break
		case messages.CandidateBlock:
			s.MineAdmin <- msg // send candidate block from blockchain to miner
			//s.NetAdmin <- msg  // send candidate block to be broadcast to network
			break
		case messages.StopMine:
			s.MineAdmin <- msg // signal miner to stop
			break
		case messages.ShareBlock:
			//s.NetAdmin <- msg // send verified block to be broadcase to network
			break
		case messages.Transaction:
			s.BcAdmin <- msg
			break
		case messages.GenCandidate:
			s.BcAdmin <- msg
			break
		case messages.ReqHeight:
			s.BcAdmin <- msg
			break
		case messages.Height:
			s.Info <- msg
			break
		}
	}
}

// Close closes all channels
func (s *Server) Close() {
	close(s.WalAdmin)
	close(s.MineAdmin)
	close(s.BcAdmin)
	close(s.NetAdmin)
	close(s.Serv)
}
