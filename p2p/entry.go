package p2p

import (
	"encoding/gob"
	"log"
	"math/rand"
	"os"
)

func AppendEntryAddr(addr string) {
	path := EntryAddrsPath()

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal("fatal: could not create root wallet, ", err)
	}
	defer file.Close()

	err = gob.NewEncoder(file).Encode(addr)
	if err != nil {
		log.Fatal("fatal: could not write entry address to file, ", err)
	}
}

func ReadEntryAddrs() []string {
	path := EntryAddrsPath()

	file, err := os.Open(path)
	if err != nil {
		log.Fatal("fatal: could not open root wallet, ", err)
	}
	defer file.Close()

	var addr, buf string
	var addrs []string
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&buf)
	if err != nil {
		log.Fatal("fatal: could not read entry addresses, ", err)
	}
	for err == nil {
		addr = string([]byte(buf))
		addrs = append(addrs, addr)
		err = decoder.Decode(&buf)
	}

	return addrs
}

func RandomEntryAddr() string {
	addrs := ReadEntryAddrs()

	if len(addrs) == 0 {
		log.Fatalln("fatal: could not find entry addresses")
	}
	log.Println(addrs, "..", len(addrs))
	return addrs[rand.Intn(len(addrs))]
}

func EntryAddrsPath() string {
	path := os.Getenv("_I32COIN_ENTRYADDRS_PATH")
	if path == "" {
		log.Fatal("fatal: could not locate entry addresses path")
	}
	return path
}
