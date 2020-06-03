package p2p

import (
	"bufio"
	"fmt"
	"log"
	"net"

	"github.com/JMWorden/int32coin/blockchain"
)

func connect(host string, port string) {
	hostport := fmt.Sprintf("%s:%s", host, port)

	conn, err := net.Dial("tcp", hostport)
	if err != nil {
		log.Fatal("tcp client fatal: ", err)
	}

	// now connected
	message := bufio.NewReader(conn)

	_, err = blockchain.Recv(message)
	if err != nil {
		log.Println("unabled to read block from host, error: ", err)
	}
}
