package blockchain

import (
	"math/rand"
)

func FindNonce(b *Block) (*Block, error) {
	// start at a random nonce
	b.Nonce = uint64(rand.Int63())

	for ok, err := b.HashOk(); !ok; ok, err = b.HashOk() {
		if err != nil {
			return nil, err
		}
		b.Nonce++
	}

	return b, nil
}
