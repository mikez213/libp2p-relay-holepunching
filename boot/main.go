package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/routing"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/config"
	multiaddr "github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("bootstrap")

var RelayerPrivateKeys = []string{
	"CAESQAA7xVQKsQ5VAC5ge+XsixR7YnDkzuHa4nrY8xWXGK3fo9yN1Eaiat9Vn1iwaVQDqTjywVP303ojVLxXcQ9ze4E=",
	//12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n
	"CAESQMCYbjRpXBDUnIpDyqY+mA3n7z9gF3CaggWTknd90LauHUcz8ldNtlUchFATmMSE1r/NMnSpEBbLvzWQKq3N45s=",
	//12D3KooWBnext3VBZZuBwGn3YahAZjf49oqYckfx64VpzH6dyU1p
	"CAESQB1Y1Li0Wd4KcvMvbv5/+CTG79axzl3R8yTuzWOckMgmNAzZqxim5E/7e9mgd87FTMPQNHqiItqTFwHJeMxr0H8=",
	//12D3KooWDKYjXDDgSGzhEYWYtDvfP9pMtGNY1vnAwRsSp2CwCWHL
}

const DefaultRelayerKeyIndex = 0

func RelayIdentity(keyIndex int) func(*config.Config) error {
	return func(cfg *config.Config) error {
		if keyIndex < 0 || keyIndex >= len(RelayerPrivateKeys) {
			return fmt.Errorf("invalid key index: %d", keyIndex)
		}
		relayerPrivateKey := RelayerPrivateKeys[keyIndex]

		b, err := crypto.ConfigDecodeKey(relayerPrivateKey)
		if err != nil {
			return fmt.Errorf("failed to decode relayer private key: %v", err)
		}

		priv, err := crypto.UnmarshalPrivateKey(b)
		if err != nil {
			return fmt.Errorf("failed to unmarshal private key: %v", err)
		}
		return cfg.Apply(libp2p.Identity(priv))
	}
}

func main() {
	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("bootstrap", "debug")

	listenPort := flag.Int("port", 1237, "TCP port to listen on")
	bootstrapPeers := flag.String("bootstrap", "", "Comma-separated list of bootstrap peer multiaddresses")
	keyIndex := flag.Int("key", DefaultRelayerKeyIndex, "Index of the RelayerPrivateKey to use")
	flag.Parse()

	// Validate key index
	if *keyIndex < 0 || *keyIndex >= len(RelayerPrivateKeys) {
		log.Fatalf("Invalid key index %d. Must be between 0 and %d", *keyIndex, len(RelayerPrivateKeys)-1)
	}

	ctx := context.Background()
	host, err := libp2p.New(
		RelayIdentity(*keyIndex), // Use selected RelayerPrivateKey
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", *listenPort)),
		libp2p.EnableRelay(),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
		libp2p.ForceReachabilityPublic(),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) { return dht.New(ctx, h, dht.Mode(dht.ModeServer)) }),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Bootstrap node is running. We are: %s", host.ID())
	log.Info("Listening on:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}

	kademliaDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}

	if *bootstrapPeers != "" {
		peers := strings.Split(*bootstrapPeers, ",")
		for _, peerAddr := range peers {
			peerAddr = strings.TrimSpace(peerAddr)
			if peerAddr == "" {
				log.Warnf("No addr")
				continue
			}

			maddr, err := multiaddr.NewMultiaddr(peerAddr)
			if err != nil {
				log.Errorf("Invalid bootstrap peer multiaddress '%s': %v", peerAddr, err)
				continue
			}

			peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
			if err != nil {
				log.Errorf("Failed to get peer info from address '%s': %v", peerAddr, err)
				continue
			}

			// Add peer addresses to the peerstore
			host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)

			// Attempt to connect to the bootstrap peer
			if err := host.Connect(ctx, *peerInfo); err != nil {
				log.Errorf("Failed to connect to bootstrap peer %s: %v", peerInfo.ID, err)
				continue
			}
			log.Infof("Connected to bootstrap peer %s", peerInfo.ID)
		}
	}

	time.Sleep(2 * time.Second)

	log.Infof("Running. Peer ID: %s", host.ID())
	log.Info("Use multiaddresses to connect:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}

	go func() {
		for {
			peers := kademliaDHT.RoutingTable().ListPeers()
			log.Infof("Current routing table peers (%d): %v", len(peers), peers)
			time.Sleep(5 * time.Second)
		}
	}()

	select {}
}
