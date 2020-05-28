package blockchain

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
)

const initQLen int = 32
const shaHashSize int = 32

// Blockchain is the main structure that references all the blocks and contains global info
type Blockchain struct {
	height uint64            // number of blocks in the block chain
	blocks map[uint64]*Block // blocks in the block chain, indexed by height
	queued []Transaction     // transactions not in any block
}

// NewBlockchain creates a new block chain with genesis block
func NewBlockchain() (*Blockchain, error) {
	bc := Blockchain{height: 0, blocks: make(map[uint64]*Block), queued: make([]Transaction, 0, initQLen)}

	gen, err := genesisBlock()
	if err != nil {
		return nil, err
	}
	bc.blocks[gen.Height] = gen

	return &bc, nil
}

// creates first block
func genesisBlock() (*Block, error) {
	gen := Block{Height: 0, PrevHash: make([]byte, 32), Transactions: make([]Transaction, 0)}

	gen.MerkleRoot = make([]byte, shaHashSize)

	diff, err := strconv.Atoi(os.Getenv("_I32COIN_DIFFICULTY"))
	if err != nil {
		return nil, err
	}

	gen.Target = make([]byte, shaHashSize)
	for i := 0; i < shaHashSize; i++ {
		if i < diff {
			gen.Target[i] = 0xFF
		} else {
			gen.Target[i] = 0x0
		}
	}

	return &gen, err
}

// Top returns top of blockchain
func (bc *Blockchain) Top() *Block {
	return bc.blocks[bc.height]
}

// Enqueue validates and enqueues a transaction to be added to the block chain
func (bc *Blockchain) Enqueue(t Transaction) error {
	err := bc.validateTransaction(t)

	if err != nil {
		t.Seq = uint32(len(bc.queued))
		bc.queued = append(bc.queued, t)
		log.Printf("%v\n", bc.queued)
	} else {
		log.Println("error: ", err)
	}

	return err
}

func (bc *Blockchain) validateTransaction(t Transaction) error {
	err := bc.validateBalance(t.Sender, t.Amount)

	if err != nil {
		err = t.ValidateSignature()
	}

	return err
}

func (bc *Blockchain) validateBalance(sender Hash, amount uint32) error {
	var bal int64 = 0

	for h := uint64(0); h <= bc.height; h++ {
		block := bc.blocks[h]
		for _, trans := range block.Transactions {
			if trans.Sender.Equals(sender) {
				bal -= int64(trans.Amount)
			} else if trans.Reciever.Equals(sender) {
				bal += int64(trans.Amount)
			}
		}
	}

	for _, trans := range bc.queued {
		if trans.Sender.Equals(sender) {
			bal -= int64(trans.Amount)
		} else if trans.Reciever.Equals(sender) {
			bal += int64(trans.Amount)
		}
	}

	if bal < int64(amount) {
		str := fmt.Sprintf("balance is %v, tried to send %v", bal, amount)
		return errors.New(str)
	}
	return nil
}

// CandidateBlock copies queue into a new block and returns the block
func (bc *Blockchain) CandidateBlock() (*Block, error) {
	top := bc.blocks[bc.height]
	prevHash, err := top.Hash()
	if err != nil {
		return nil, err
	}

	qCopy := make([]Transaction, len(bc.queued))
	copy(qCopy, bc.queued)

	block, err := NewBlock(bc.height+1, prevHash, qCopy)
	if err != nil {
		return nil, err
	}

	return block, nil
}

// AddBlock validates integrity of block, adding to blockchain if legitimate
func (bc *Blockchain) AddBlock(b *Block, miner Hash) bool {
	ok, err := b.HashOk()
	added := false
	if ok && err == nil && bc.valuesOk(b) && bc.transactionsOk(b, miner) {
		bc.height++
		bc.blocks[bc.height] = b
		bc.purgeQueued(b.Transactions)
		added = true
	}
	return added
}

func (bc *Blockchain) purgeQueued(transactions []Transaction) {
	included := make(map[string]bool, len(transactions))

	for _, trans := range transactions {
		included[trans.String()] = true
	}

	newQueue := make([]Transaction, 0, len(bc.queued))

	// add transactions that still aren't in any block and are valid
	seq := uint32(1)
	for _, trans := range bc.queued {
		_, found := included[trans.String()]
		if !found {
			err := bc.validateTransaction(trans)
			if err == nil {
				trans.Seq = seq
				newQueue = append(newQueue, trans)
				seq++
			}
		}
	}

	bc.queued = newQueue
}

func (bc *Blockchain) valuesOk(b *Block) bool {
	ok := true
	top := bc.blocks[bc.height]
	prevHash, err := top.Hash()
	if err != nil {
		log.Println("error:", err)
		ok = false
	}

	// validate previous hash is the same
	if ok {
		ok = b.PrevHash.Equals(prevHash)
	}

	// validate target is the same
	if ok {
		ok = b.Target.Equals(top.Target)
	}

	// validate merkle root is the same
	if ok {
		ok = b.MerkleRoot.Equals(top.MerkleRoot)
	}
	return ok
}

func (bc *Blockchain) transactionsOk(b *Block, miner Hash) bool {
	ok := true
	root, err := calcMerkleRoot(b.Transactions)
	if err != nil {
		log.Println("error: ", err)
		ok = false
	}

	if ok { // validate the merkle root
		ok = root.Equals(b.MerkleRoot)
	}

	if ok { // validate each transaction
		for i, trans := range b.Transactions {
			if i == 0 {
				continue // skip the reward
			}
			err := bc.validateTransaction(trans)
			if err != nil {
				ok = false
				break
			}
		}
	}

	if b.Height != 0 && len(b.Transactions) < 1 {
		log.Println("empty transactions")
		ok = false
	}

	if ok { // validate the reward
		reward := b.Transactions[0]
		ok = reward.Reciever.Equals(miner) && reward.Sender.Equals(Hash([]byte{0})) &&
			reward.Signature.Equals(Hash([]byte{0}))
	}

	return ok
}
