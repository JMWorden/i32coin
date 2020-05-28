package main

import (
	"bytes"
	"fmt"
	"log"

	"./blockchain"
	"./wallet"
)

func main() {

	wall, err := wallet.NewWallet()
	if err != nil {
		log.Println("error: ", err)
		return
	}

	fmt.Println(wall)

	bc, err := blockchain.NewBlockchain()
	if err != nil {
		log.Println("error:", err)
	} else {
		gen := bc.Top()
		fmt.Println(gen)
		hash, _ := gen.Hash()
		fmt.Println("hash: ", hash)
		buf := new(bytes.Buffer)
		err := gen.Send(buf)
		if err != nil {
			log.Println("error: ", err)
			return
		}

		newBlock, err := blockchain.Recv(buf)
		if err != nil {
			log.Println("error: ", err)
			return
		}
		fmt.Println(newBlock)

		blockchain.FindNonce(newBlock)
		fmt.Println(newBlock)
		fmt.Println(newBlock.Hash())

		//bc.Enqueue(blockchain.Transaction{0, 1, 1})
	}

	trans := blockchain.NewTransaction(wall.Addr, wall.Addr, 1)
	err = trans.Sign(wall.Priv)
	if err != nil {
		log.Println("error: ", err)
	}
	fmt.Println("transaction: ", trans)

	err = trans.ValidateSignature()
	if err != nil {
		log.Println("error: ", err)
	}

}
