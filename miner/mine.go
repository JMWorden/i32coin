package miner

import (
	"log"
	"math/rand"

	"../blockchain"
	"../wallet"
)

// increments nonce until working hash is found
func findNonce(b *blockchain.Block) (*blockchain.Block, error) {
	b.Nonce = uint64(rand.Int63())

	for ok, err := b.HashOk(); !ok; ok, err = b.HashOk() {
		if err != nil {
			return nil, err
		}
		b.Nonce++
	}

	return b, nil
}

// Create reward transaction from 0x0 to miner for reward amount
func makeReward(b *blockchain.Block, w *wallet.Wallet) blockchain.Transaction {
	sender := blockchain.RootHash()
	amount := blockchain.RewardAmount()
	trans := blockchain.NewTransaction(sender, w.Addr, amount)
	trans.Seq = 0
	trans.Signature = blockchain.RootHash()

	return trans
}

// Mine tries to generate proof of work for candidate block, added reward as first transaction
func Mine(b *blockchain.Block, w *wallet.Wallet, bc *blockchain.Blockchain) {
	b.Transactions = append([]blockchain.Transaction{makeReward(b, w)}, b.Transactions...)

	root, err := blockchain.CalcMerkleRoot(b.Transactions)
	if err != nil {
		log.Fatal("fatal: ", err)
	}
	b.MerkleRoot = root

	// start at a random nonce
	b.Nonce = uint64(w.RandGen.Int63())
	_, err = findNonce(b)
	if err != nil {
		log.Fatal("fatal: ", err)
	}

	bc.AddBlock(b, w.Addr)
}
