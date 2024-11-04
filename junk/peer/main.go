package main

import (
	"context"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"

	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("chatlog")

func main() {
	// Set up logging levels
	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("chatnode", "debug")

	ctx := context.Background()

	host, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
	)
	if err != nil {
		log.Fatalf("Failed to create libp2p host: %v", err)
	}

	bootstrapAddrStr := "/ip4/127.0.0.1/tcp/1237/p2p/12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n"
	bootstrapMaddr, err := multiaddr.NewMultiaddr(bootstrapAddrStr)
	if err != nil {
		log.Fatalf("Invalid bootstrap multiaddress: %v", err)
	}

	bootstrapPeerInfo, err := peer.AddrInfoFromP2pAddr(bootstrapMaddr)
	if err != nil {
		log.Fatalf("Failed to parse bootstrap peer info: %v", err)
	}
	bootstrapPeers := []peer.AddrInfo{*bootstrapPeerInfo}

	log.Infof("Bootstrap peer: %s", bootstrapPeerInfo.ID)

	if err := host.Connect(ctx, bootstrapPeers[0]); err != nil {
		log.Fatalf("Failed to connect to bootstrap peer: %v", err)
	}
	log.Infof("Connected to bootstrap peer: %s", bootstrapPeers[0].ID)

	kademliaDHT, err := dht.New(ctx, host,
		dht.Mode(dht.ModeServer),
		dht.BootstrapPeers(bootstrapPeers...),
	)
	if err != nil {
		log.Fatalf("Failed to create DHT: %v", err)
	}

	log.Info("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatalf("DHT bootstrap failed: %v", err)
	}

	log.Infof("Host created with ID: %s", host.ID())

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
