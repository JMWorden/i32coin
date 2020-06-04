package main

import (
	"bufio"
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/JMWorden/int32coin/blockchain"
	"github.com/JMWorden/int32coin/messages"
	"github.com/JMWorden/int32coin/miner"
	"github.com/JMWorden/int32coin/p2p"
	"github.com/JMWorden/int32coin/router"
	"github.com/JMWorden/int32coin/wallet"
)

func main() {
	port := flag.Int("port", -1, "listen port number")
	target := flag.String("peer", "", "target peer to dial")
	genrw := flag.Bool("genrw", false, "generate root wallet")
	flag.Parse()

	if *port == -1 {
		log.Fatal("No port provided with -l")
	}

	if *genrw {
		genRootWallet()
	}

	s, w := startSystem(1, *port, *target)

	//testSystem(s, w)
	interactiveTestSystem(s, w)

	waitForSignal(s)
}

func genRootWallet() {
	w := wallet.NewWallet()
	path := rootWalletPath()

	file, err := os.Create(path)
	if err != nil {
		log.Fatal("fatal: could not create root wallet, ", err)
	}
	defer file.Close()

	err = gob.NewEncoder(file).Encode(w)
	if err != nil {
		log.Fatal("fatal: could not write root wallet to file, ", err)
	}
}

func readRootWallet() *wallet.Wallet {
	path := rootWalletPath()

	file, err := os.Open(path)
	if err != nil {
		log.Fatal("fatal: could not open root wallet, ", err)
	}
	defer file.Close()

	w := wallet.Wallet{}
	err = gob.NewDecoder(file).Decode(&w)
	if err != nil {
		log.Fatal("fatal: could not write root wallet to file, ", err)
	}

	return &w
}

func rootWalletPath() string {
	path := os.Getenv("_I32COIN_ROOTWALL_PATH")
	if path == "" {
		log.Fatal("fatal: could not locate root wallet path")
	}
	return path
}

func startSystem(amount uint32, port int, target string) (*router.Router, *wallet.Wallet) {
	r := router.NewRouter()

	w := readRootWallet()
	first := blockchain.NewTransaction(blockchain.RootHash(), w.Addr, 1, 0)
	err := first.Sign(w.Priv)
	if err != nil {
		log.Fatal("fatal: failed to create sign first transaction: ")
	}
	bc := blockchain.NewBlockchain(first)
	m := miner.NewMiner(w)

	p2p.Init(port, r.NetAdmin, r.Serv)
	if target == "" {
		go p2p.Genesis()
	} else {
		go p2p.Peer(target)
	}

	go r.Route()
	go m.Listen(r.MineAdmin, r.Serv)
	go bc.Listen(r.BcAdmin, r.Serv)

	return r, w
}

func waitForSignal(server *router.Router) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig
	server.Close()
}

func testSystem(r *router.Router, w *wallet.Wallet) {
	wall1 := wallet.NewWallet()
	trans1 := blockchain.NewTransaction(w.Addr, wall1.Addr, 1, 1)
	trans1.Sign(w.Priv)

	r.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans1}
	r.Serv <- messages.LocalMsg{Mtype: messages.GenCandidate}

	wall2 := wallet.NewWallet()
	wall3 := wallet.NewWallet()

	trans2 := blockchain.NewTransaction(w.Addr, wall2.Addr, 5, 2)
	trans2.Sign(w.Priv)
	trans3 := blockchain.NewTransaction(w.Addr, wall3.Addr, 7, 2)
	trans3.Sign(w.Priv)

	time.Sleep(400 * time.Millisecond)

	r.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans2}
	r.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans3}
	r.Serv <- messages.LocalMsg{Mtype: messages.GenCandidate}

	trans4 := blockchain.NewTransaction(wall2.Addr, wall3.Addr, 1, 3)
	trans4.Sign(wall2.Priv)
	trans5 := blockchain.NewTransaction(wall3.Addr, wall1.Addr, 8, 3)
	trans5.Sign(wall3.Priv)
	trans6 := blockchain.NewTransaction(w.Addr, wall3.Addr, 7, 3)
	trans6.Sign(w.Priv)

	time.Sleep(400 * time.Millisecond)

	r.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans4}
	r.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans5}
	r.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans6}
	r.Serv <- messages.LocalMsg{Mtype: messages.GenCandidate}
}

func interactiveTestSystem(r *router.Router, w *wallet.Wallet) {
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
			r.Serv <- messages.LocalMsg{Mtype: messages.ReqHeight}
			heightMsg := <-r.Info
			trans := blockchain.NewTransaction(from.Addr, to.Addr, uint32(amount), heightMsg.Height+1)
			trans.Sign(from.Priv)
			r.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans}
			break
		case "post":
			r.Serv <- messages.LocalMsg{Mtype: messages.GenCandidate}
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
