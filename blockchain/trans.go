package blockchain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/asn1"
	"errors"
	"fmt"
	"io"
	"math/big"
)

// Transaction is transaction in the block chain
type Transaction struct {
	Seq       uint32 // sequence number in block, reward has seq of 0
	Sender    Hash   // public key of sender (wallet addr)
	Reciever  Hash   // public key of reciever (wallet addr)
	Amount    uint32 // amount of i32coins
	Signature Hash   // signature of sender
}

// NewTransaction generates new transaction without a seq or signature
func (t Transaction) NewTransaction(pub *ecdsa.PublicKey, reciever Hash, amount uint32) Transaction {
	sender := elliptic.Marshal(pub, pub.X, pub.Y)
	return Transaction{Sender: sender, Reciever: reciever, Amount: amount}
}

// Sign generates signature for transaction using sender, reciever, and amount
func (t Transaction) Sign(priv *ecdsa.PrivateKey, rand io.Reader) ([]byte, error) {
	digestHash, err := t.hashDigest()
	if err != nil {
		return nil, err
	}
	return priv.Sign(rand, digestHash, nil)
}

func (t Transaction) String() string {
	return fmt.Sprintf("%v,%v,%v,%v,%v", t.Seq, t.Sender, t.Reciever, t.Amount, t.Signature)
}

// Double hash
func (t Transaction) hash() (Hash, error) {
	str := t.String()

	sha := sha256.New()
	if _, err := sha.Write([]byte(str)); err != nil {
		return nil, err
	}

	first := sha.Sum(nil)
	sha = sha256.New()
	if _, err := sha.Write(first); err != nil {
		return nil, err
	}

	return sha.Sum(nil), nil
}

// only hashes sender, reciever, and amount (twice)
func (t Transaction) hashDigest() (Hash, error) {
	str := fmt.Sprintf("%v,%v,%v", t.Sender, t.Reciever, t.Amount)

	sha := sha256.New()
	if _, err := sha.Write([]byte(str)); err != nil {
		return nil, err
	}

	first := sha.Sum(nil)
	sha = sha256.New()
	if _, err := sha.Write(first); err != nil {
		return nil, err
	}

	return sha.Sum(nil), nil
}

// Equals returns true if both transactions have the same values
func (t Transaction) Equals(other Transaction) bool {
	return t.Sender.Equals(other.Sender) && t.Reciever.Equals(other.Reciever) && t.Seq == other.Seq &&
		t.Amount == other.Amount && t.Signature.Equals(other.Signature)
}

// ValidateSignature validates transaction was signed by the sender
func (t Transaction) ValidateSignature() error {
	var pub ecdsa.PublicKey
	elliptic.Unmarshal(pub, t.Sender)

	hash, err := t.hashDigest()
	if err != nil {
		return err
	}

	sig := struct{ r, s *big.Int }{}
	_, err = asn1.Unmarshal(t.Signature, &sig)

	if !ecdsa.Verify(&pub, hash, sig.r, sig.s) {
		return errors.New("signature invalid")
	}

	return nil
}
