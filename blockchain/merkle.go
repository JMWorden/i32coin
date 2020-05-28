package blockchain

import (
	"crypto/sha256"

	"github.com/cbergoon/merkletree"
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

func calcMerkleRoot(transactions []Transaction) (Hash, error) {
	nodes := make([]merkletree.Content, len(transactions))

	for t, trans := range transactions {
		nodes[t] = merkleLeaf{trans}
	}
	tree, err := merkletree.NewTree(nodes)
	if err != nil {
		return nil, err
	}

	first := tree.MerkleRoot()
	sha := sha256.New()
	if _, err := sha.Write(first); err != nil {
		return nil, err
	}

	return Hash(sha.Sum(nil)), nil
}
