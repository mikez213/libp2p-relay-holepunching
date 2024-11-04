package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"
)

var log = logging.Logger("bootlog")

var relayerPrivateKeys = []string{
	"CAESQAA7xVQKsQ5VAC5ge+XsixR7YnDkzuHa4nrY8xWXGK3fo9yN1Eaiat9Vn1iwaVQDqTjywVP303ojVLxXcQ9ze4E=",
}

const defaultRelayerKeyIndex = 0

func RelayIdentity(keyIndex int) (libp2p.Option, error) {
	if keyIndex < 0 || keyIndex >= len(relayerPrivateKeys) {
		return nil, fmt.Errorf("invalid key index: %d", keyIndex)
	}

	keyStr := relayerPrivateKeys[keyIndex]
	keyBytes, err := crypto.ConfigDecodeKey(keyStr)
	if err != nil {
		return nil, fmt.Errorf("decode private key failed: %w", err)
	}

	privKey, err := crypto.UnmarshalPrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("unmarshal key failed: %w", err)
	}

	return libp2p.Identity(privKey), nil
}

func main() {
	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("bootstrap", "debug")

	listenPort := flag.Int("port", 1237, "TCP port to listen on")
	keyIndex := flag.Int("key", defaultRelayerKeyIndex, "Relayer private key index")
	flag.Parse()

	relayOpt, err := RelayIdentity(*keyIndex)
	if err != nil {
		log.Fatalf("relay id err: %v", err)
	}

	ctx := context.Background()
	host, err := libp2p.New(
		relayOpt,
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *listenPort)),
		libp2p.ForceReachabilityPublic(),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Bootstrap node up with PID: %s", host.ID())
	log.Info("Listening on:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}

	kademliaDHT, err := dht.New(ctx, host)
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("Bootstrapping DHT")
	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}

	time.Sleep(2 * time.Second)

	log.Infof("Running PID: %s", host.ID())
	log.Info("Use multiaddrs to connect:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}

	go func() {
		for {
			if err := kademliaDHT.RefreshRoutingTable(); err != nil {
				log.Errorf("Error refreshing routing table: %v", err)
			}
			peers := kademliaDHT.RoutingTable().ListPeers()
			log.Infof("Routing table peers (%d): %v", len(peers), peers)
			time.Sleep(10 * time.Second)
		}
	}()

	select {}
}
