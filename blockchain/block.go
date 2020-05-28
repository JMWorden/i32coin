package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

// Hash is sha256 array of bytes
type Hash []byte

func (h Hash) Equals(other Hash) bool {
	equal := true
	if len(h) == len(other) {
		for i, byt := range h {
			if byt != other[i] {
				equal = false
				break
			}
		}
	} else {
		equal = false
		log.Println("warning: hashes of different legnths")
	}
	return equal
}

func (h Hash) String() string {
	return hex.EncodeToString(h)
}

// Block is block on the block chain
type Block struct {
	Height       uint64        // height of this block
	Nonce        uint64        // value that miners are incrementing
	PrevHash     Hash          // hash of previous block
	MerkleRoot   Hash          // merkle root of transaction merkle tree
	Target       Hash          // hash should be less than this value
	Transactions []Transaction // transactions in this block
}

// NewBlock generates a new block wil default nonce
func NewBlock(height uint64, prevHash Hash, transactions []Transaction) (*Block, error) {
	b := Block{Height: height, PrevHash: prevHash, Transactions: transactions}

	merkleRoot, err := calcMerkleRoot(b.Transactions)
	if err != nil {
		return nil, err
	}
	b.MerkleRoot = merkleRoot

	diff, err := strconv.Atoi(os.Getenv("_I32COIN_DIFFICULTY"))
	if err != nil {
		return nil, err
	}

	b.Target = make([]byte, sha256.Size)
	for i := 0; i < sha256.Size; i++ {
		if i < diff {
			b.Target[i] = 0xFF
		} else {
			b.Target[i] = 0x0
		}
	}

	return &b, nil
}

// Double hash
func (b *Block) Hash() (Hash, error) {
	sha := sha256.New()

	if _, err := sha.Write(b.PrevHash); err != nil {
		return nil, err
	}
	if _, err := sha.Write(b.MerkleRoot); err != nil {
		return nil, err
	}

	nonceBuf := new(bytes.Buffer)
	binary.Write(nonceBuf, binary.LittleEndian, b.Nonce)
	if _, err := sha.Write(nonceBuf.Bytes()); err != nil {
		return nil, err
	}

	first := sha.Sum(nil)
	sha = sha256.New()
	if _, err := sha.Write(first); err != nil {
		return nil, err
	}

	return sha.Sum(nil), nil
}

func (b *Block) String() string {
	return fmt.Sprintf("block %v: \n\tnonce:%v\n\tprevHash:%v\n\troot:%v\n\ttarget:%v\n\ttrans:%v\n",
		b.Height, b.Nonce, b.PrevHash, b.MerkleRoot, b.Target, b.Transactions)
}

// Send encodes Block and transmits to io.Writer (assumedly the network)
func (b *Block) Send(w io.Writer) error {
	encoder := gob.NewEncoder(w)

	err := encoder.Encode(b)
	if err != nil {
		log.Println("enocde error: ", err)
	}

	return err
}

// Recv decodes Block from io.Reader (assumedly the network)
func Recv(r io.Reader) (*Block, error) {
	b := Block{}
	decoder := gob.NewDecoder(r)

	err := decoder.Decode(&b)
	if err != nil {
		log.Println("decode error: ", err)
	}

	return &b, err
}

// Bytes converts block to byte array in network order (big endian)
func (b *Block) Bytes() ([]byte, error) {
	buf := new(bytes.Buffer)

	err := binary.Write(buf, binary.BigEndian, b)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// HashOk returns true if hash is less than target hash
func (b *Block) HashOk() (bool, error) {
	hash, err := b.Hash()
	if err != nil {
		log.Println("error: ", err)
		return false, err
	}

	ok := true
	for i := len(hash) - 1; i > -1; i-- {
		if hash[i] > b.Target[i] {
			ok = false
			break
		}
	}

	return ok, nil
}
