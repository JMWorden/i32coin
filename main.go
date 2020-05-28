package main

import (
	"log"
	"os"
	"os/signal"
	"time"

	"./int32coin"
	"./int32coin/blockchain"
	"./int32coin/miner"
	"./int32coin/network"
	"./int32coin/wallet"
)

func main() {
	/*
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
		//m.Mine(b)
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
		//m.Mine(b) // block 2

		fmt.Println(b)
	*/
	s, w := startSystem(1)

	testSystem(s, w)

	waitForSignal(s)
}

func startSystem(amount uint32) (*network.Server, *wallet.Wallet) {
	s := network.NewServer()
	w := wallet.NewWallet()
	first := blockchain.NewTransaction(blockchain.RootHash(), w.Addr, 1)
	err := first.Sign(w.Priv)
	if err != nil {
		log.Fatal("fatal server init failure: ")
	}
	bc := blockchain.NewBlockchain(first)
	m := miner.NewMiner(w)

	go s.Route()
	go m.Listen(s.MineAdmin, s.Serv)
	go bc.Listen(s.BcAdmin, s.Serv)

	return s, w
}

func waitForSignal(server *network.Server) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	server.Close()
}

func testSystem(s *network.Server, w *wallet.Wallet) {
	wall1 := wallet.NewWallet()
	trans1 := blockchain.NewTransaction(w.Addr, wall1.Addr, 1)
	trans1.Sign(w.Priv)

	s.Serv <- int32coin.LocalMsg{Mtype: int32coin.Transaction, Transaction: trans1}
	s.Serv <- int32coin.LocalMsg{Mtype: int32coin.GenCandidate}

	wall2 := wallet.NewWallet()
	wall3 := wallet.NewWallet()

	trans2 := blockchain.NewTransaction(w.Addr, wall2.Addr, 5)
	trans2.Sign(w.Priv)
	trans3 := blockchain.NewTransaction(w.Addr, wall3.Addr, 7)
	trans3.Sign(w.Priv)

	time.Sleep(400 * time.Millisecond)

	s.Serv <- int32coin.LocalMsg{Mtype: int32coin.Transaction, Transaction: trans2}
	s.Serv <- int32coin.LocalMsg{Mtype: int32coin.Transaction, Transaction: trans3}
	s.Serv <- int32coin.LocalMsg{Mtype: int32coin.GenCandidate}

	trans4 := blockchain.NewTransaction(wall2.Addr, wall3.Addr, 1)
	trans4.Sign(wall2.Priv)
	trans5 := blockchain.NewTransaction(wall3.Addr, wall1.Addr, 8)
	trans5.Sign(wall3.Priv)
	trans6 := blockchain.NewTransaction(w.Addr, wall3.Addr, 7)
	trans6.Sign(w.Priv)

	time.Sleep(400 * time.Millisecond)

	s.Serv <- int32coin.LocalMsg{Mtype: int32coin.Transaction, Transaction: trans4}
	s.Serv <- int32coin.LocalMsg{Mtype: int32coin.Transaction, Transaction: trans5}
	s.Serv <- int32coin.LocalMsg{Mtype: int32coin.Transaction, Transaction: trans6}
	s.Serv <- int32coin.LocalMsg{Mtype: int32coin.GenCandidate}
}

func interactiveTestSystem(s *network.Server, w *wallet.Wallet) {

}
