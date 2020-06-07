package blockchain

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/JMWorden/int32coin/messages"
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
func NewBlockchain(first Transaction) *Blockchain {
	bc := Blockchain{height: 0, blocks: make(map[uint64]*Block), queued: make([]Transaction, 0, initQLen)}
	bc.blocks[0] = genesisBlock(first)
	return &bc
}

// creates first block
func genesisBlock(first Transaction) *Block {
	gen := Block{Height: 0, PrevHash: make([]byte, 32), Transactions: make([]Transaction, 1)}
	gen.Transactions[0] = first
	gen.Target = makeTarget()

	root, err := CalcMerkleRoot(gen.Transactions)
	if err != nil {
		log.Fatal("blockchain fatal: failed to create genesis block: ", err)
	}
	gen.MerkleRoot = root

	return &gen
}

// Listen listens for messages and processes them
func (bc *Blockchain) Listen(in <-chan messages.LocalMsg, out chan<- messages.LocalMsg) {
	for msg := range in {
		switch msg.Mtype {
		case messages.AddBlock:
			log.Println("blockchain: inspecting block ", msg.Block.(*Block).Height)
			added := bc.addBlock(msg.Block.(*Block))
			if added {
				log.Printf("blockchain: sharing block")
				out <- messages.LocalMsg{Mtype: messages.ShareBlock, Block: bc.Top()}
			} else {
				log.Printf("blockchain: skipping block")
			}
			if len(bc.queued) > 0 {
				// generate candidate block
				b := bc.CandidateBlock()
				out <- messages.LocalMsg{Mtype: messages.CandidateBlock, Block: b}
			}
			break
		case messages.Transaction:
			bc.Enqueue(msg.Transaction.(Transaction))
			break
		case messages.GenCandidate:
			b := bc.CandidateBlock()
			log.Println("sending candidate")
			out <- messages.LocalMsg{Mtype: messages.CandidateBlock, Block: b}
			break
		case messages.ReqHeight:
			out <- messages.LocalMsg{Mtype: messages.Height, Height: bc.height}
			break
		case messages.RemoveBlocks:
			bc.removeBlocks(msg.Height)
			break
		case messages.RangeReq:
			out <- messages.LocalMsg{Mtype: messages.Range, Block: bc.getRange(msg.Height)}
			break
		}
	}
}

func (bc *Blockchain) removeBlocks(first uint64) {
	for h := first; h <= bc.height; h++ {
		log.Println("blockchain: removing block ", h)
		delete(bc.blocks, h)
	}
	bc.height = first - 1
}

func (bc *Blockchain) getRange(first uint64) []*Block {
	blocks := make([]*Block, bc.height-first+1)

	for h, ndx := first, 0; h <= bc.height; h++ {
		blocks[ndx] = bc.blocks[h]
		ndx++
	}

	return blocks
}

// RootHash returns all 0 hash; used for rewards and default signature
func RootHash() Hash {
	return Hash(make([]byte, shaHashSize))
}

// Top returns top of blockchain
func (bc *Blockchain) Top() *Block {
	return bc.blocks[bc.height]
}

// Enqueue validates and enqueues a transaction to be added to the block chain
func (bc *Blockchain) Enqueue(t Transaction) error {
	t.Seq = uint32(len(bc.queued)) + 1
	err := bc.validateTransaction(t, bc.queued)

	if err != nil {
		log.Println("blockchain: queue rejects bad transaction: ", err)
	} else {
		bc.queued = append(bc.queued, t)
		log.Println("blockchain: queued transaction #", t.Seq)
	}

	return err
}

// Validates sender has sufficient balance and transaction was properly signed
func (bc *Blockchain) validateTransaction(t Transaction, external []Transaction) error {
	err := bc.validateBalance(t.Sender, t.Amount, t.Seq, external)

	if err == nil {
		err = t.ValidateSignature()
	}

	if err == nil && t.Sender.Equals(t.Reciever) {
		err = errors.New("sender and reciever are the same")
	}

	if err == nil && t.Height != bc.height+1 {
		str := fmt.Sprintf("transaction has bad block height %v", t.Height)
		err = errors.New(str)
	}

	return err
}

// Validates sender has sufficient balance (looks at block history and queue)
func (bc *Blockchain) validateBalance(sender Hash, amount uint32, seq uint32,
	external []Transaction) error {
	var bal int64 = 0
	var err error = nil

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

	for _, trans := range external {
		if trans.Seq == seq { // ignore this transaction
			continue // TODO : why is this transaction even in the queue yet?
		}
		if trans.Sender.Equals(sender) {
			bal -= int64(trans.Amount)
		} else if trans.Reciever.Equals(sender) {
			bal += int64(trans.Amount)
		}
	}

	if bal < int64(amount) {
		str := fmt.Sprintf("balance is %v, tried to send %v", bal, amount)
		err = errors.New(str)
	}
	return err
}

// CandidateBlock copies queue into a new block and returns the block
func (bc *Blockchain) CandidateBlock() *Block {
	top := bc.blocks[bc.height]
	prevHash, err := top.Hash()
	if err != nil {
		log.Fatal("blockchain fatal: ", err)
	}

	qCopy := make([]Transaction, len(bc.queued))
	copy(qCopy, bc.queued)

	return NewBlock(bc.height+1, prevHash, qCopy)
}

// addBlock validates integrity of block, adding to blockchain if legitimate
func (bc *Blockchain) addBlock(b *Block) bool {
	added := false

	if b.Height != bc.height+1 {
		log.Println("blockchain: block height mismatch")
	} else {
		ok, err := b.HashOk()
		if !ok {
			log.Println("blockchain: block hash not ok")
		}
		if ok && err == nil && bc.valuesOk(b) && bc.transactionsOk(b) {
			bc.height++
			bc.blocks[bc.height] = b
			bc.purgeQueued(b.Transactions)
			added = true
			log.Printf("blockchain: added block %v\n", bc.height)
		}
	}

	return added
}

func (bc *Blockchain) purgeQueued(transactions []Transaction) {
	included := make(map[string]bool, len(transactions))

	// visit transactions that were just added to the block chain
	for _, trans := range transactions {
		included[trans.String()] = true
	}

	oldQueue := bc.queued
	bc.queued = make([]Transaction, 0, len(oldQueue))

	// add transactions that still aren't in any block and are valid
	seq := uint32(1)
	for _, trans := range oldQueue {
		_, found := included[trans.String()]
		if !found {
			err := bc.validateTransaction(trans, bc.queued)
			if err == nil {
				trans.Seq = seq
				bc.queued = append(bc.queued, trans)
				seq++
			}
		}
		log.Printf("blockchain: keeping %d transactions after adding block", len(bc.queued))
	}
}

// Returns true if target and previous hash match expected
func (bc *Blockchain) valuesOk(b *Block) bool {
	ok := true
	top := bc.blocks[bc.height]
	prevHash, err := top.Hash()
	if err != nil {
		log.Fatal("blockchain fatal:", err)
		ok = false
	}

	// validate previous hash is the same
	if ok {
		ok = b.PrevHash.Equals(prevHash)
		if !ok {
			log.Println("blockchain: bad block -- previous block hash mismatch")
		}
	}

	// validate target is the same
	if ok {
		ok = b.Target.Equals(makeTarget())
		if !ok {
			log.Println("blockchain: bad block -- target hash mismatch")
		}
	}

	return ok
}

func (bc *Blockchain) transactionsOk(b *Block) bool {
	ok := true
	root, err := CalcMerkleRoot(b.Transactions)
	if err != nil {
		log.Println("blockchain: bad block -- ", err)
		ok = false
	}

	if ok { // validate the merkle root
		ok = root.Equals(b.MerkleRoot)
		if !ok {
			log.Println("blockchain: bad transaction -- merkle root mismatch")
		}
	}

	if ok { // validate each transaction
		for _, trans := range b.Transactions {
			if trans.Seq == 0 {
				continue // skip the reward
			}
			err := bc.validateTransaction(trans, b.Transactions)
			if err != nil {
				log.Printf("blockchain: bad transaction (#%v) -- %v", trans.Seq, err)
				ok = false
				break
			}
		}
	}

	if b.Height != 0 && len(b.Transactions) < 2 {
		log.Println("blockchain: bad block -- empty transactions (ignoring reward)")
		ok = false
	}

	if ok { // validate the reward
		reward := b.Transactions[0]
		ok = reward.Sender.Equals(RootHash()) && reward.Signature.Equals(RootHash()) &&
			reward.Amount == RewardAmount()
		if !ok {
			log.Println("blockchain: bad block -- reward incorrect")
		}
	}

	return ok
}

// RewardAmount returns expected reward amount
func RewardAmount() uint32 {
	amount, err := strconv.Atoi(os.Getenv("_I32COIN_REWARD"))
	if err != nil {
		log.Fatal("blockchain fatal: could not determine expected reward")
	}
	return uint32(amount)
}
