package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("chatlog")

var bootstrapPeerIDs = []peer.ID{}

var relayerPrivateKeys = []string{
	//boots
	"CAESQAA7xVQKsQ5VAC5ge+XsixR7YnDkzuHa4nrY8xWXGK3fo9yN1Eaiat9Vn1iwaVQDqTjywVP303ojVLxXcQ9ze4E=",
	// pid: 12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n
	"CAESQMCYbjRpXBDUnIpDyqY+mA3n7z9gF3CaggWTknd90LauHUcz8ldNtlUchFATmMSE1r/NMnSpEBbLvzWQKq3N45s=",
	// pid: 12D3KooWBnext3VBZZuBwGn3YahAZjf49oqYckfx64VpzH6dyU1p
	"CAESQB1Y1Li0Wd4KcvMvbv5/+CTG79axzl3R8yTuzWOckMgmNAzZqxim5E/7e9mgd87FTMPQNHqiItqTFwHJeMxr0H8=",
	// pid: 12D3KooWDKYjXDDgSGzhEYWYtDvfP9pMtGNY1vnAwRsSp2CwCWHL

	//relays
	"CAESQHMEeM3iNIIxNThxIfnuO5FJ0oUQJy8V7TFD80lGziBE7SuPw2wckCrFRihVDaw0e6PkDCwsh/6u3UgBxB3OTFo=",
	//12D3KooWRnBKUEkAEpsoCoEiuhxKBJ5j2Bdop6PGxFMvd4PwoevM
	"CAESQP3Pu7TVp2RSVIZykj65/MDXm/eiTOfLGH3xCWQVmUoC67MkFWUEOd6QERl1Y4Xvi1Rt+d36UuaFXanT+hVUDAY=",
	//12D3KooWRgSQnguL2DYkXUXqCLiRQ35PEX4eEH3havy2X18AVALd
	"CAESQDE2IToG5mWwzWEeXt3/OVbx9XyE743DTenPFUG8M06IQXSarkNhuxNEJisnWeuDvaoaM/fNJNMqhPR81NL3Pio=",
	//12D3KooWEDso33ti9KsKmD2g2egNmw6BXgch7V5vFz1TziuNYybo

	//nodes
	"CAESQFffsVM3eUXLozmXkBM2FSSVhEmo/Cq5RlXOAAaniTdCu3EQ6Zf7lQDasCj6IXyTihFQWZB+nmGFn/ZAA5y5egk=",
	//12D3KooWNS4QQxwNURwoYoXmGjH9AQkagcGTjRUQT33P4i4FKQsi
	"CAESQCSHrfyzNZkxwoNmXI1wx5Lvr6o4+kGxGepFH0AfYlKthyON+1hQRjLJQaBAQLrr1cfMHFFoC40X62DQIhL246U=",
	//12D3KooWJuteouY1d5SYFcAUAYDVPjFD8MUBgqsdjZfBkAecCS2Y
	"CAESQDyiSqC9Jez8wKSQs74YJalAegamjVKHbnaN35pfe6Gk21WVgCzfvBdLVoRj8XXny/k1LtSOhPZWNz0rWKCOYpk=",
	//12D3KooWQaZ9Ppi8A2hcEspJhewfPqKjtXu4vx7FQPaUGnHXWpNL
}

func parseBootstrap(bootstrapAddrs []string) []peer.AddrInfo {
	var bootstrapPeers []peer.AddrInfo
	for _, addrStr := range bootstrapAddrs {
		addrStr = strings.TrimSpace(addrStr)
		if addrStr == "" {
			continue
		}
		maddr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			log.Errorf("invalid bootstrap addr '%s': %v", addrStr, err)
			continue
		}
		peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Errorf("failed to parse bootstrap peer info from '%s': %v", addrStr, err)
			continue
		}
		bootstrapPeers = append(bootstrapPeers, *peerInfo)
	}

	return bootstrapPeers
}

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
	// logging.SetAllLoggers(logging.LevelDebug)
	logging.SetAllLoggers(logging.LevelInfo)
	// logging.SetLogLevel("dht", "error") // supresss warn and info
	logging.SetLogLevel("chatnode", "debug")

	// var kademliaDHT *dht.IpfsDHT
	host, err := libp2p.New(
		// nodeOpt,
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		// libp2p.EnableRelay(),
		// // libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{*relayInfo}, autorelay.WithMetricsTracer(mt)),
		// libp2p.NATPortMap(),
		// libp2p.EnableRelayService(),
		// libp2p.EnableNATService(),
		// libp2p.EnableAutoNATv2(),
		// libp2p.EnableHolePunching(),
		// libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
		// 	kademliaDHT, _ = dht.New(ctx, h, dht.Mode(dht.ModeClient))
		// 	return kademliaDHT, nil
		// }),
	)
	if err != nil {
		log.Fatalf("failed to create libp2p host: %v", err)
	}

	ctx := context.Background()
	// var bootstrapPeers := boot_id
	// for i, addr := range bootstrapPeers {
	// 	peerinfo, _ := peer.AddrInfoFromP2pAddr(addr)
	// 	bootstrapPeers[i] = addr
	// }

	// addrStr := "/ip4/18.195.99.129/tcp/8000/p2p/12D3KooWC2Mu6XPbWenZYg4EmRR8gVERrytiUnmwkiH8u9MbuviC"
	addrStr := "/ip4/127.0.0.1/tcp/1237/p2p/12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n"

	// addrStr := "/ip4/127.0.0.1/tcp/12354"
	addr, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		log.Fatal(err)
		return
	}
	peerinfo, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		log.Fatal(err)
		return
	}
	bootstrapPeers := []peer.AddrInfo{*peerinfo}
	log.Error(bootstrapPeers)

	kademliaDHT, err := dht.New(ctx, host, dht.BootstrapPeers(bootstrapPeers...))
	if err != nil {
		panic(err)
	}
	log.Error("dht c")

	log.Debug("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		panic(err)
	}

	log.Infof("host created, we are %s", host.ID())

	go func() {
		for {
			kademliaDHT.RefreshRoutingTable() //has a channel to block, but unused for now
			peers := kademliaDHT.RoutingTable().ListPeers()
			log.Infof("Routing table peers (%d): %v", len(peers), peers)
			time.Sleep(10 * time.Second)
		}
	}()
	select {}
}
