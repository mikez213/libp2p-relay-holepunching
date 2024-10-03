package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/routing"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/network"
)

var log = logging.Logger("bootstrap")

const RelayerPrivateKey = "CAESQAA7xVQKsQ5VAC5ge+XsixR7YnDkzuHa4nrY8xWXGK3fo9yN1Eaiat9Vn1iwaVQDqTjywVP303ojVLxXcQ9ze4E="

var RelayIdentity = func(cfg *config.Config) error {
	b, err := crypto.ConfigDecodeKey(RelayerPrivateKey)
	if err != nil {
		return err
	}

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

}

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			log.Error("Error reading from buffer:", err)
			return
		}

		if str != "" && str != "\n" {
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
			log.Error("Error reading from stdin:", err)
			return
		}

		_, err = rw.WriteString(sendData)
		if err != nil {
			log.Error("Error writing to buffer:", err)
			return
		}
		err = rw.Flush()
		if err != nil {
			log.Error("Error flushing buffer:", err)
			return
		}
	}
}

func main() {
	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("bootstrap", "debug")

	listenPort := 1237

	ctx := context.Background()
	host, err := libp2p.New(
		RelayIdentity,
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)),
		libp2p.EnableRelay(),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
		libp2p.ForceReachabilityPublic(),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) { return dht.New(ctx, h, dht.Mode(dht.ModeServer)) }),
		// libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) { return dht.New(ctx, h) }),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Bootstrap node is running. We are: %s", host.ID())
	log.Info("Listening on:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}

	host.SetStreamHandler("/chat/1.0.0", handleStream)

	kademliaDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}

	time.Sleep(5 * time.Second)

	log.Infof("running. Peer ID: %s", host.ID())
	log.Info("Use multiaddresses to connect:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}

	select {}
}
