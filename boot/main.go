package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"
	multiaddr "github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("bootstrap")

// encoded private keys for relayers
var relayerPrivateKeys = []string{
	"CAESQAA7xVQKsQ5VAC5ge+XsixR7YnDkzuHa4nrY8xWXGK3fo9yN1Eaiat9Vn1iwaVQDqTjywVP303ojVLxXcQ9ze4E=",
	// 12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n
	"CAESQMCYbjRpXBDUnIpDyqY+mA3n7z9gF3CaggWTknd90LauHUcz8ldNtlUchFATmMSE1r/NMnSpEBbLvzWQKq3N45s=",
	// 12D3KooWBnext3VBZZuBwGn3YahAZjf49oqYckfx64VpzH6dyU1p
	"CAESQB1Y1Li0Wd4KcvMvbv5/+CTG79axzl3R8yTuzWOckMgmNAzZqxim5E/7e9mgd87FTMPQNHqiItqTFwHJeMxr0H8=",
	// 12D3KooWDKYjXDDgSGzhEYWYtDvfP9pMtGNY1vnAwRsSp2CwCWHL
}

const defaultRelayerKeyIndex = 0

// RelayIdentity sets the host's identity based on the key index
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
	// setup logging
	logging.SetAllLoggers(logging.LevelWarn)
	logging.SetLogLevel("bootstrap", "debug")

	// parse flags
	listenPort := flag.Int("port", 1237, "tcp port to listen on")
	bootstrapPeers := flag.String("bootstrap", "", "comma-separated bootstrap peer multiaddrs")
	keyIndex := flag.Int("key", defaultRelayerKeyIndex, "relayer private key index")
	flag.Parse()

	// set identity
	relayOpt, err := RelayIdentity(*keyIndex)
	if err != nil {
		log.Fatalf("relay identity error: %v", err)
	}

	// create host
	ctx := context.Background()
	host, err := libp2p.New(
		relayOpt,
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *listenPort)),
		libp2p.EnableRelay(),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
		libp2p.ForceReachabilityPublic(),
	)
	if err != nil {
		log.Fatal(err)
	}

	// display host info
	log.Infof("bootstrap node running. peer ID: %s", host.ID())
	log.Info("listening on:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}

	// setup DHT
	kademliaDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("bootstrapping DHT")
	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}

	// connect to bootstrap peers
	if *bootstrapPeers != "" {
		peerAddrs := strings.Split(*bootstrapPeers, ",")
		for _, addr := range peerAddrs {
			addr = strings.TrimSpace(addr)
			if addr == "" {
				log.Warn("empty bootstrap peer address")
				continue
			}

			maddr, err := multiaddr.NewMultiaddr(addr)
			if err != nil {
				log.Errorf("invalid bootstrap peer addr '%s': %v", addr, err)
				continue
			}

			peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
			if err != nil {
				log.Errorf("get peer info failed for '%s': %v", addr, err)
				continue
			}

			// add to peerstore
			host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)

			// connect
			if err := host.Connect(ctx, *peerInfo); err != nil {
				log.Errorf("connect to bootstrap peer %s failed: %v", peerInfo.ID, err)
				continue
			}
			log.Infof("connected to bootstrap peer %s", peerInfo.ID)
		}
	}

	// wait for connections
	time.Sleep(2 * time.Second)

	// show connection info
	log.Infof("running. peer ID: %s", host.ID())
	log.Info("use multiaddrs to connect:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}

	// log routing table
	go func() {
		for {
			peers := kademliaDHT.RoutingTable().ListPeers()
			log.Infof("routing table peers (%d): %v", len(peers), peers)
			time.Sleep(10 * time.Second)
		}
	}()

	select {}
}
