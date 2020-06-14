package main

import (
	"bufio"
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"math/rand"
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
	genroot := flag.Bool("genroot", false, "generate root wallet & transaction")
	auto := flag.Bool("auto", false, "automatically peer")
	appendHost := flag.Bool("append-host", false, "append host address to entry point file")
	nopeer := flag.Bool("nopeer", false, "append address to entry point file")
	flag.Parse()

	if *port == -1 {
		log.Fatal("No port provided with -port")
	}

	if *target == "" && !*auto && !*nopeer {
		log.Fatal("Must specify entry point or use automatic peering for non-entry point")
	}

	if *genroot {
		genRootTransaction(genRootWallet())
	}

	s, w := startSystem(10, *port, *target, *auto, *appendHost, *nopeer)

	interactiveTestSystem(s, w)

	waitForSignal(s)
}

func genRootWallet() *wallet.Wallet {
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

	return w
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

func genRootTransaction(rootWallet *wallet.Wallet) {
	t := blockchain.NewTransaction(blockchain.RootHash(), rootWallet.Addr, 1)
	err := t.Sign(rootWallet.Priv)
	if err != nil {
		log.Fatal("fatal: failed to create sign root transaction: ")
	}
	path := rootTransactionPath()

	file, err := os.Create(path)
	if err != nil {
		log.Fatal("fatal: could not create root transaction, ", err)
	}
	defer file.Close()

	err = gob.NewEncoder(file).Encode(t)
	if err != nil {
		log.Fatal("fatal: could not write root transaction to file, ", err)
	}
}

func readRootTransaction() blockchain.Transaction {
	path := rootTransactionPath()

	file, err := os.Open(path)
	if err != nil {
		log.Fatal("fatal: could not open root transaction, ", err)
	}
	defer file.Close()

	t := blockchain.Transaction{}
	err = gob.NewDecoder(file).Decode(&t)
	if err != nil {
		log.Fatal("fatal: could not write root transaction to file, ", err)
	}

	return t
}

func rootTransactionPath() string {
	path := os.Getenv("_I32COIN_ROOTTRANS_PATH")
	if path == "" {
		log.Fatal("fatal: could not locate root transaction path")
	}
	return path
}

func startSystem(amount uint32, port int, target string,
	auto bool, appendHost bool, nopeer bool) (*router.Router, *wallet.Wallet) {
	r := router.NewRouter()

	w := readRootWallet()
	first := readRootTransaction()
	bc := blockchain.NewBlockchain(first)
	m := miner.NewMiner(w)

	p2p.Init(port, r.NetAdmin, r.Serv)

	if appendHost {
		p2p.AppendEntryAddr(p2p.HostAddr())
	}

	if nopeer {
		go p2p.Genesis()
	} else {
		if auto {
			go p2p.Peer(p2p.RandomEntryAddr())
		} else {
			go p2p.Peer(target)
		}
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
			trans := blockchain.NewTransaction(from.Addr, to.Addr, uint32(amount))
			trans.Sign(from.Priv)
			r.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans}
			break
		case "post":
			r.Serv <- messages.LocalMsg{Mtype: messages.GenCandidate}
			break
		case "rand":
			randomTransactions(r, w)
			fmt.Printf("done with random transactions")
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

func randomTransactions(r *router.Router, mw *wallet.Wallet) {
	randSrc := rand.New(rand.NewSource(time.Now().UnixNano()))
	wallets := make([]*wallet.Wallet, randSrc.Intn(10)+1)
	for w := range wallets {
		wallets[w] = wallet.NewWallet()
	}

	for _, w := range wallets {
		trans := blockchain.NewTransaction(mw.Addr, w.Addr, uint32(randSrc.Intn(1)+1))
		trans.Sign(mw.Priv)
		r.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans}
	}

	log.Println("done with loop of randoms")

	from := wallets[randSrc.Intn(len(wallets))]
	to := wallets[randSrc.Intn(len(wallets))]
	trans := blockchain.NewTransaction(from.Addr, to.Addr, uint32(1))
	err := trans.Sign(from.Priv)
	if err != nil {
		log.Println("random trans error: ,", err)
		return
	}
	r.Serv <- messages.LocalMsg{Mtype: messages.Transaction, Transaction: trans}
}
