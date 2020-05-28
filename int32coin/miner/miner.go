package miner

import (
	"log"
	"math/rand"

	"../../int32coin"
	"../blockchain"
	"../wallet"
)

// Miner will mine blocks using wallet for reward destination
type Miner struct {
	w *wallet.Wallet
}

// NewMiner creates a new miner with the pass wallet
func NewMiner(w *wallet.Wallet) *Miner {
	m := Miner{w: w}
	return &m
}

// Listen listens for candidate blocks from server
func (m *Miner) Listen(in <-chan int32coin.LocalMsg, out chan<- int32coin.LocalMsg) {
	for msg := range in {
		switch msg.Mtype {
		case int32coin.CandidateBlock:
			m.Mine(msg.Block.(*blockchain.Block), in, out)
			break
		}
	}
}

// Mine tries to generate proof of work for candidate block, added reward as first transaction
func (m *Miner) Mine(b *blockchain.Block, in <-chan int32coin.LocalMsg, out chan<- int32coin.LocalMsg) {
	b.Transactions = append([]blockchain.Transaction{m.makeReward(b)}, b.Transactions...)

	root, err := blockchain.CalcMerkleRoot(b.Transactions)
	if err != nil {
		log.Fatal("fatal: ", err)
	}
	b.MerkleRoot = root

	// start at a random nonce
	b.Nonce = uint64(m.w.RandGen.Int63())
	mined, err := findNonce(b, in)
	if err != nil {
		log.Println("error: ", err)
	} else if mined != nil {
		out <- int32coin.LocalMsg{Mtype: int32coin.AddBlock, Block: b, Miner: m.w.Addr}
	}
}

// Create reward transaction from 0x0 to miner for reward amount
func (m *Miner) makeReward(b *blockchain.Block) blockchain.Transaction {
	sender := blockchain.RootHash()
	amount := blockchain.RewardAmount()
	trans := blockchain.NewTransaction(sender, m.w.Addr, amount, b.Height)
	trans.Seq = 0
	trans.Signature = blockchain.RootHash()

	return trans
}

// increments nonce until working hash is found
func findNonce(b *blockchain.Block, in <-chan int32coin.LocalMsg) (*blockchain.Block, error) {
	b.Nonce = uint64(rand.Int63())

	for ok, err := b.HashOk(); !ok; ok, err = b.HashOk() {
		select { // non-blocking check for stop
		case msg := <-in:
			if msg.Mtype == int32coin.StopMine {
				return nil, nil
			}
			break
		default:
			break
		}

		if err != nil {
			return nil, err
		}
		b.Nonce++
	}

	return b, nil
}
