package main

import (
	"log"

	"./blockchain"
	"./miner"
	"./wallet"
)

func main() {

	wall1 := wallet.NewWallet()
	wall2 := wallet.NewWallet()

	// properly signed
	trans := blockchain.NewTransaction(wall1.Addr, wall2.Addr, 1)
	err := trans.Sign(wall1.Priv)
	if err != nil {
		log.Println("error: ", err)
	}

	err = trans.ValidateSignature()
	if err != nil {
		log.Println("error: ", err)
	}

	bc := blockchain.NewBlockchain(trans)

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

	b := bc.CandidateBlock()
	miner.Mine(b, wall2, bc)
}
