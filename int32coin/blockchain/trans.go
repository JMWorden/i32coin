package blockchain

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"golang.org/x/crypto/sha3"
)

// Transaction is transaction in the block chain
type Transaction struct {
	Seq       uint32 // sequence number in block, reward has seq of 0
	Sender    Hash   // public key of sender (wallet addr)
	Reciever  Hash   // public key of reciever (wallet addr)
	Amount    uint32 // amount of i32coins
	Signature Hash   // signature of sender
	Height    uint64
}

// NewTransaction generates new transaction without a seq or signature
func NewTransaction(sender Hash, reciever Hash, amount uint32, height uint64) Transaction {
	return Transaction{Sender: sender, Reciever: reciever, Amount: amount, Height: height}
}

// Sign generates signature for transaction digest (sender, reciever, amount, and height)
func (t *Transaction) Sign(priv Hash) error {
	digest, err := t.digest()
	if err != nil {
		return err
	}

	sig, err := secp256k1.Sign(digest, priv)
	if err != nil {
		return err
	}
	t.Signature = sig

	return nil
}

func (t *Transaction) String() string {
	return fmt.Sprintf("%v,%v,%v,%v,%v,%v", t.Seq, t.Sender, t.Reciever, t.Amount, t.Signature, t.Height)
}

// doulbe hashs all fields (sha3-256)
func (t *Transaction) hash() (Hash, error) {
	str := t.String()

	sha := sha3.New256()
	if _, err := sha.Write([]byte(str)); err != nil {
		return nil, err
	}

	first := sha.Sum(nil)
	sha = sha3.New256()
	if _, err := sha.Write(first); err != nil {
		return nil, err
	}

	return sha.Sum(nil), nil
}

func (t *Transaction) predigest() Hash {
	return []byte(fmt.Sprintf(fmt.Sprintf("%v,%v,%v,%v", t.Sender, t.Reciever, t.Amount, t.Height)))
}

// only (double sha3-256) hashes sender, reciever, amount, and height (twice)
func (t *Transaction) digest() (Hash, error) {
	sha := sha3.New256()
	if _, err := sha.Write(t.predigest()); err != nil {
		return nil, err
	}

	first := sha.Sum(nil)
	sha = sha3.New256()
	if _, err := sha.Write(first); err != nil {
		return nil, err
	}

	return sha.Sum(nil), nil
}

// Equals returns true if both transactions have the same values
func (t *Transaction) Equals(other Transaction) bool {
	return t.Sender.Equals(other.Sender) && t.Reciever.Equals(other.Reciever) && t.Seq == other.Seq &&
		t.Amount == other.Amount && t.Signature.Equals(other.Signature)
}

// ValidateSignature validates transaction was signed by the sender
func (t *Transaction) ValidateSignature() error {
	digest, err := t.digest()
	if err != nil {
		return err
	}

	// get public key of signature
	sigpub, err := crypto.SigToPub(digest, t.Signature)
	if err != nil {
		return err
	}
	sigpubHash := Hash(crypto.FromECDSAPub(sigpub))

	// convert signature public key to address
	sha := sha3.New256()
	sha.Write(sigpubHash)
	addr := Hash(sha.Sum(nil))

	if !addr.Equals(t.Sender) {
		return errors.New("signature invalid")
	}

	return nil
}
