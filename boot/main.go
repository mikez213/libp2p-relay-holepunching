package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"

	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/network"
)

var log = logging.Logger("bootstrap")

const RelayerPrivateKey = "CAESQAA7xVQKsQ5VAC5ge+XsixR7YnDkzuHa4nrY8xWXGK3fo9yN1Eaiat9Vn1iwaVQDqTjywVP303ojVLxXcQ9ze4E="

var RelayIdentity = func(cfg *config.Config) error {
	b, _ := crypto.ConfigDecodeKey(RelayerPrivateKey)

	priv, err := crypto.UnmarshalPrivateKey(b)
	if err != nil {
		return err
	}
	return cfg.Apply(libp2p.Identity(priv))
}

func handleStream(stream network.Stream) {
	log.Info("Got a new stream!")

	// Create a buffer stream for non-blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go readData(rw)
	go writeData(rw)

	// 'stream' will stay open until you close it (or the other side closes it).
}

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			log.Error("Error reading from buffer: ", err)
			return
		}

		if str == "" {
			return
		}
		if str != "\n" {
			// Green console color: 	\x1b[32m
			// Reset console color: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}
	}
}

func writeData(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			log.Error("Error reading from stdin: ", err)
			return
		}

		_, err = rw.WriteString(fmt.Sprintf("%s\n", sendData))
		if err != nil {
			log.Error("Error writing to buffer: ", err)
			return
		}
		err = rw.Flush()
		if err != nil {
			log.Error("Error flushing buffer: ", err)
			return
		}
	}
}

func main() {
	// Initialize logging
	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("bootstrap", "debug")

	// Parse command-line flags
	// listenPort := flag.Int("port", 0, "Port to listen on")
	// flag.Parse()
	listenPort := 1237
	// if *listenPort == 0 {
	// 	fmt.Println("Please provide a port to bind on with -port")
	// 	os.Exit(1)
	// }

	// Create a new libp2p Host
	ctx := context.Background()
	host, err := libp2p.New(
		RelayIdentity,
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)),
		libp2p.EnableRelay(),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Bootstrap node is running. We are:", host.ID())
	log.Info(host.Addrs())

	// Set stream handler
	host.SetStreamHandler("/chat/1.0.0", handleStream)

	// Start DHT without bootstrap peers
	kademliaDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		log.Fatal(err)
	}

	// Bootstrap the DHT
	log.Debug("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}

	// Wait for the DHT to bootstrap
	time.Sleep(5 * time.Second)

	fmt.Println("Bootstrap node is running. Peer ID:", host.ID())
	fmt.Println("Use the following multiaddress to connect:")
	for _, addr := range host.Addrs() {
		fmt.Printf("%s/p2p/%s\n", addr, host.ID())
	}

	// Keep the process running
	select {}
}
