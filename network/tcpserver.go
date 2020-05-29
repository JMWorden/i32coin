package network

/*
func start(port string) (host.Host, error) {
	var rdr io.Reader = rand.New(rand.NewSource(time.Now().UnixNano()))

	// generate key pair for host
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rdr)
	if err != nil {
		log.Println("could not generate private key")
		return nil, err
	}

	listenAddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%s", port)
	opts := []libp2p.Option{libp2p.ListenAddStrings(listenAddr),
		libp2p.Identity(priv), libp2p.NoEncryption()}

	host, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		log.Println("could not create p2p host")
		return nil, err
	}

	addr, _ := ma.NewMultiaddr()
}
*/
