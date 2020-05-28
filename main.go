package main

import (
	"fmt"
	"log"

	"./blockchain"
	"./miner"
	"./wallet"
)

func main() {

	wall0 := wallet.NewWallet()
	wall1 := wallet.NewWallet()
	wall2 := wallet.NewWallet()

	// properly signed
	trans := blockchain.NewTransaction(wall0.Addr, wall1.Addr, 1)
	err := trans.Sign(wall0.Priv)

	bc := blockchain.NewBlockchain(trans)
	fmt.Println("top: ", bc.Top())

	// improperly signed
	badTrans := blockchain.NewTransaction(wall1.Addr, wall2.Addr, 1)
	err = badTrans.Sign(wall2.Priv)
	if err != nil {
		log.Println("error: ", err)
	}

	err = badTrans.ValidateSignature()
	if err == nil {
		log.Fatal("fatal: accepted bad signature")
	}

	trans1 := blockchain.NewTransaction(wall1.Addr, wall0.Addr, 1)
	err = trans1.Sign(wall1.Priv)
	bc.Enqueue(trans1)

	b := bc.CandidateBlock() // block 1
	miner.Mine(b, wall2, bc)
	fmt.Println(b)

	wall3 := wallet.NewWallet()

	trans2 := blockchain.NewTransaction(wall2.Addr, wall1.Addr, 1)
	trans2.Sign(wall2.Priv)
	bc.Enqueue(trans2)
	trans3 := blockchain.NewTransaction(wall2.Addr, wall3.Addr, 4)
	trans3.Sign(wall2.Priv)
	bc.Enqueue(trans3)
	trans4 := blockchain.NewTransaction(wall3.Addr, wall1.Addr, 2)
	trans4.Sign(wall2.Priv) // bad signature
	bc.Enqueue(trans4)
	trans5 := blockchain.NewTransaction(wall1.Addr, wall2.Addr, 1)
	trans5.Sign(wall1.Priv)
	bc.Enqueue(trans5)

	b = bc.CandidateBlock()
	miner.Mine(b, wall3, bc) // block 2

	fmt.Println(b)
}
