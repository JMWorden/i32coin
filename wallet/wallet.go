package wallet

import (
	"crypto/ecdsa"
	"fmt"
	"log"

	"golang.org/x/crypto/sha3"

	"github.com/JMWorden/int32coin/blockchain"
	"github.com/ethereum/go-ethereum/crypto"
)

// Wallet is contains public/private key for address
type Wallet struct {
	Priv         blockchain.Hash
	Pub          blockchain.Hash          // public key
	Addr         blockchain.Hash          // address derived from public key
	Transactions []blockchain.Transaction // transactions sent/recieved from this wallet
}

// NewWallet creates a new wallet with a public/private key pair (and address)
func NewWallet() *Wallet {
	w := Wallet{Transactions: make([]blockchain.Transaction, 0)}

	// generate private key
	priv, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}
	privHash := blockchain.Hash(crypto.FromECDSA(priv))
	w.Priv = privHash

	// generate public key
	pub, ok := priv.Public().(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cast to public key failed")
	}
	pubHash := blockchain.Hash(crypto.FromECDSAPub(pub))
	w.Pub = pubHash

	// generate address
	hash := sha3.New256()
	hash.Write(pubHash)
	addr := hash.Sum(nil)
	w.Addr = addr

	return &w
}

func (w *Wallet) String() string {
	return fmt.Sprintf("wallet:\n\tpriv:%v\n\taddr:%v\n\ttrans:%v", w.Priv, w.Addr, w.Transactions)
}
