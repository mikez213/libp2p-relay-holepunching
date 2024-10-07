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

// RelayerPrivateKeys holds the encoded private keys for relayers
var relayerPrivateKeys = []string{
	"CAESQAA7xVQKsQ5VAC5ge+XsixR7YnDkzuHa4nrY8xWXGK3fo9yN1Eaiat9Vn1iwaVQDqTjywVP303ojVLxXcQ9ze4E=",
	// 12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n
	"CAESQMCYbjRpXBDUnIpDyqY+mA3n7z9gF3CaggWTknd90LauHUcz8ldNtlUchFATmMSE1r/NMnSpEBbLvzWQKq3N45s=",
	// 12D3KooWBnext3VBZZuBwGn3YahAZjf49oqYckfx64VpzH6dyU1p
	"CAESQB1Y1Li0Wd4KcvMvbv5/+CTG79axzl3R8yTuzWOckMgmNAzZqxim5E/7e9mgd87FTMPQNHqiItqTFwHJeMxr0H8=",
	// 12D3KooWDKYjXDDgSGzhEYWYtDvfP9pMtGNY1vnAwRsSp2CwCWHL
}

const defaultRelayerKeyIndex = 0

// RelayIdentity returns a libp2p.Option to set the host's identity
func RelayIdentity(keyIndex int) (libp2p.Option, error) {
	if keyIndex < 0 || keyIndex >= len(relayerPrivateKeys) {
		return nil, fmt.Errorf("invalid key index: %d", keyIndex)
	}

	relayerPrivateKey := relayerPrivateKeys[keyIndex]
	keyBytes, err := crypto.ConfigDecodeKey(relayerPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode relayer private key: %w", err)
	}

	privKey, err := crypto.UnmarshalPrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal private key: %w", err)
	}

	return libp2p.Identity(privKey), nil
}

func main() {
	// Initialize logging
	logging.SetAllLoggers(logging.LevelWarn)
	logging.SetLogLevel("bootstrap", "debug")

	// Parse command-line flags
	listenPort := flag.Int("port", 1237, "TCP port to listen on")
	bootstrapPeers := flag.String("bootstrap", "", "Comma-separated list of bootstrap peer multiaddresses")
	keyIndex := flag.Int("key", defaultRelayerKeyIndex, "Index of the RelayerPrivateKey to use")
	flag.Parse()

	// Set host identity
	relayOption, err := RelayIdentity(*keyIndex)
	if err != nil {
		log.Fatalf("Failed to set relay identity: %v", err)
	}

	// Create a new libp2p host
	ctx := context.Background()
	host, err := libp2p.New(
		relayOption,
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

	// Display host information
	log.Infof("Bootstrap node is running. Peer ID: %s", host.ID())
	log.Info("Listening on:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}

	// Initialize and bootstrap the DHT in server mode
	kademliaDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("Bootstrapping the DHT")
	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}

	// Connect to bootstrap peers if provided
	if *bootstrapPeers != "" {
		peerAddrs := strings.Split(*bootstrapPeers, ",")
		for _, peerAddr := range peerAddrs {
			peerAddr = strings.TrimSpace(peerAddr)
			if peerAddr == "" {
				log.Warn("Empty bootstrap peer address")
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

	// Allow some time for connections to establish
	time.Sleep(2 * time.Second)

	// Display connection information
	log.Infof("Running. Peer ID: %s", host.ID())
	log.Info("Use multiaddresses to connect:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}

	// Periodically log the current routing table peers
	go func() {
		for {
			peers := kademliaDHT.RoutingTable().ListPeers()
			log.Infof("Current routing table peers (%d): %v", len(peers), peers)
			time.Sleep(5 * time.Second)
		}
	}()

	// Keep the program running indefinitely
	select {}
}
