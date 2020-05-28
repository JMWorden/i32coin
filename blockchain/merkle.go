package blockchain

import (
	"github.com/cbergoon/merkletree"
	"golang.org/x/crypto/sha3"
)

// MerkleNode implements merkletree.Content interface
type merkleLeaf struct {
	trans Transaction
}

// CalculateHash is required by merkletree.Content interface
func (l merkleLeaf) CalculateHash() ([]byte, error) {
	return l.trans.hash()
}

// Equals is required by merkletree.Content interface
func (l merkleLeaf) Equals(other merkletree.Content) (bool, error) {
	return l.trans.Equals(other.(merkleLeaf).trans), nil
}

// CalcMerkleRoot calculates root hash of merkle tree (double sha3-256)
func CalcMerkleRoot(transactions []Transaction) (Hash, error) {
	nodes := make([]merkletree.Content, len(transactions))

	for t, trans := range transactions {
		nodes[t] = merkleLeaf{trans}
	}
	tree, err := merkletree.NewTree(nodes)
	if err != nil {
		return nil, err
	}

	first := tree.MerkleRoot()
	sha := sha3.New256()
	if _, err := sha.Write(first); err != nil {
		return nil, err
	}

	return Hash(sha.Sum(nil)), nil
}
