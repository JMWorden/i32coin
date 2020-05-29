package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/JMWorden/int32coin/blockchain"
	"github.com/JMWorden/int32coin/messages"
	"github.com/JMWorden/int32coin/miner"
	"github.com/JMWorden/int32coin/network"
	"github.com/JMWorden/int32coin/wallet"
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

	//testSystem(s, w)
	interactiveTestSystem(s, w)

	waitForSignal(s)
}

func startSystem(amount uint32) (*network.Server, *wallet.Wallet) {
	s := network.NewServer()
	w := wallet.NewWallet()
	first := blockchain.NewTransaction(blockchain.RootHash(), w.Addr, 1, 0)
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
	trans1 := blockchain.NewTransaction(w.Addr, wall1.Addr, 1, 1)
	trans1.Sign(w.Priv)

	s.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans1}
	s.Serv <- messages.LocalMsg{Mtype: messages.GenCandidate}

	wall2 := wallet.NewWallet()
	wall3 := wallet.NewWallet()

	trans2 := blockchain.NewTransaction(w.Addr, wall2.Addr, 5, 2)
	trans2.Sign(w.Priv)
	trans3 := blockchain.NewTransaction(w.Addr, wall3.Addr, 7, 2)
	trans3.Sign(w.Priv)

	time.Sleep(400 * time.Millisecond)

	s.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans2}
	s.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans3}
	s.Serv <- messages.LocalMsg{Mtype: messages.GenCandidate}

	trans4 := blockchain.NewTransaction(wall2.Addr, wall3.Addr, 1, 3)
	trans4.Sign(wall2.Priv)
	trans5 := blockchain.NewTransaction(wall3.Addr, wall1.Addr, 8, 3)
	trans5.Sign(wall3.Priv)
	trans6 := blockchain.NewTransaction(w.Addr, wall3.Addr, 7, 3)
	trans6.Sign(w.Priv)

	time.Sleep(400 * time.Millisecond)

	s.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans4}
	s.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans5}
	s.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans6}
	s.Serv <- messages.LocalMsg{Mtype: messages.GenCandidate}
}

func interactiveTestSystem(s *network.Server, w *wallet.Wallet) {
	wallets := make(map[string]*wallet.Wallet)

	wallets["miner"] = w

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(bufio.ScanWords)

	fmt.Println("warning: this input method is not robust")
	fmt.Printf("$: ")
	for scanner.Scan() {
		input := scanner.Text()
		switch input {
		case "wallet":
			scanner.Scan()
			input = scanner.Text()
			wal, found := wallets[input]
			if !found {
				wal = wallet.NewWallet()
				wallets[input] = wallet.NewWallet()
				fmt.Printf("created: ")
			} else {
				fmt.Printf("found: ")
			}
			fmt.Println(wal.Addr)
			break
		case "send":
			scanner.Scan()
			from := wallets[scanner.Text()]
			scanner.Scan()
			to := wallets[scanner.Text()]
			scanner.Scan()
			amount, _ := strconv.Atoi(scanner.Text())
			s.Serv <- messages.LocalMsg{Mtype: messages.ReqHeight}
			heightMsg := <-s.Info
			trans := blockchain.NewTransaction(from.Addr, to.Addr, uint32(amount), heightMsg.Height+1)
			trans.Sign(from.Priv)
			s.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans}
			break
		case "post":
			s.Serv <- messages.LocalMsg{Mtype: messages.GenCandidate}
			break
		default:
			fmt.Println("-- invalid input")
		}
		fmt.Printf("$: ")
	}

	if scanner.Err() != nil {
		fmt.Println("-- fatal scanner error")
	}
}
