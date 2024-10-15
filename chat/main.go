package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"

	routing "github.com/libp2p/go-libp2p/core/routing"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
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

func init() {
	bootstrapIDStrs := []string{
		"12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n",
		"12D3KooWBnext3VBZZuBwGn3YahAZjf49oqYckfx64VpzH6dyU1p",
		"12D3KooWDKYjXDDgSGzhEYWYtDvfP9pMtGNY1vnAwRsSp2CwCWHL",
	}

	for _, idStr := range bootstrapIDStrs {
		pid, err := peer.Decode(idStr)
		if err != nil {
			log.Fatalf("failed to decode bootstrap pid '%s': %v", idStr, err)
		}
		bootstrapPeerIDs = append(bootstrapPeerIDs, pid)
	}
}

func isBootstrapPeer(peerID peer.ID) bool {
	for _, bootstrapID := range bootstrapPeerIDs {
		if peerID == bootstrapID {
			return true
		}
	}
	return false
}

func pingPeer(ctx context.Context, host host.Host, pid peer.ID, rendezvous string, log logging.EventLogger, connectedPeers map[peer.ID]bool) {
	stream, err := host.NewStream(ctx, pid, protocol.ID(rendezvous))
	if err != nil {
		log.Errorf("failed to open stream to %s: %v", pid, err)
		return
	}
	defer stream.Close()

	if _, err := fmt.Fprintf(stream, "PING\n"); err != nil {
		log.Errorf("failed to send ping to %s: %v", pid, err)
		return
	}
	log.Infof("sent PING to %s", pid)

	buf := make([]byte, 5)
	if _, err := stream.Read(buf); err != nil {
		log.Errorf("failed to read PONG from %s: %v", pid, err)
		return
	}

	if string(buf) == "PONG\n" {
		log.Infof("got PONG from %s", pid)
		connectedPeers[pid] = true
	} else {
		log.Warnf("unexpected resp from %s: %s", pid, string(buf))
	}
}

func handleStream(stream network.Stream) {
	log.Infof("got new stream from %s", stream.Conn().RemotePeer())

	defer stream.Close()

	buf := make([]byte, 5)
	_, err := stream.Read(buf)
	if err != nil {
		log.Errorf("err reading from stream: %v", err)
		return
	}

	received := string(buf)
	log.Infof("got msg: %s from %s", received, stream.Conn().RemotePeer())

	if received == "PING\n" {
		if _, err = fmt.Fprintf(stream, "PONG\n"); err != nil {
			log.Errorf("err writing PONG to stream: %v", err)
			return
		}
		log.Infof("sent PONG to %s", stream.Conn().RemotePeer())
	} else {
		log.Infof("unexpected msg: %s from %s", received, stream.Conn().RemotePeer())
	}
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

// func putRelayedAddress(ctx context.Context, kademliaDHT *dht.IpfsDHT, relayaddr multiaddr.Multiaddr) {
// 	key := fmt.Sprintf("/pk/%s", kademliaDHT.Host().ID())

// 	value := []byte(relayaddr.String())

// 	if err := kademliaDHT.PutValue(ctx, key, value); err != nil {
// 		log.Errorf("failed to announce relayed address %s: %v", relayaddr, err)
// 		log.Errorf("key, value: %s, %s", key, value)
// 	} else {
// 		log.Infof("announced relayed address: %s with key: %s", relayaddr, key)
// 	}
// }

// func getRelayedAddress(ctx context.Context, kademliaDHT *dht.IpfsDHT, targetPeerID peer.ID) (multiaddr.Multiaddr, error) {
// 	key := fmt.Sprintf("/pk/%s", targetPeerID)

// 	value, err := kademliaDHT.GetValue(ctx, key)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get relayed address for %s: %w", targetPeerID, err)
// 	}

// 	relayedAddr, err := multiaddr.NewMultiaddr(string(value))
// 	if err != nil {
// 		return nil, fmt.Errorf("invalid relayed address '%s': %w", value, err)
// 	}

// 	return relayedAddr, err
// }

func main() {
	logging.SetAllLoggers(logging.LevelDebug)
	// logging.SetAllLoggers(logging.LevelInfo)
	// logging.SetLogLevel("dht", "error") // supresss warn and info
	logging.SetLogLevel("chatnode", "debug")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if len(os.Args) < 3 {
		log.Fatal("need a bootstrap node and relay")
	}

	relayAddrStr := os.Args[1]
	keyIndex := os.Args[2]
	bootstrapAddrs := os.Args[3:]

	keyIndexInt, err := strconv.Atoi(keyIndex)
	if err != nil {
		log.Fatalf("index error:", err)
	}

	nodeOpt, err := RelayIdentity(keyIndexInt)
	if err != nil {
		log.Fatalf("relay identity error: %v", err)
	}

	relayMaddr, err := multiaddr.NewMultiaddr(relayAddrStr)
	if err != nil {
		log.Fatalf("bad relay address '%s': %v", relayAddrStr, err)
	}

	relayInfo, err := peer.AddrInfoFromP2pAddr(relayMaddr)
	if err != nil {
		log.Fatalf("fail to parse relay peer info from '%s': %v", relayAddrStr, err)
	}

	log.Info("relay info: ", relayInfo.ID, " address", relayInfo.Addrs)

	//parse boot string
	bootstrapPeers := parseBootstrap(bootstrapAddrs)

	if len(bootstrapPeers) == 0 {
		log.Fatal("no valid bootstrap addrs")
	}
	// relayMaddr, err := multiaddr.NewMultiaddr(relayAddrStr)
	// if err != nil {
	// 	log.Fatalf("Invalid relay address '%s': %v", relayAddrStr, err)
	// }

	// relayInfo, err := peer.AddrInfoFromP2pAddr(relayMaddr)
	// if err != nil {
	// 	log.Fatalf("Failed to parse relay peer info from '%s': %v", relayAddrStr, err)
	// }

	mt := autorelay.NewMetricsTracer()

	var kademliaDHT *dht.IpfsDHT
	host, err := libp2p.New(
		nodeOpt,
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{*relayInfo}, autorelay.WithMetricsTracer(mt)),
		libp2p.NATPortMap(),
		libp2p.EnableRelayService(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			kademliaDHT, _ = dht.New(ctx, h, dht.Mode(dht.ModeClient))
			return kademliaDHT, nil
		}),
	)
	if err != nil {
		log.Fatalf("failed to create libp2p host: %v", err)
	}

	log.Infof("host created, we are %s", host.ID())

	rendezvousString := "meetme"
	host.SetStreamHandler(protocol.ID(rendezvousString), handleStream)

	// connect to the relay node
	log.Infof("connecting to relay node %s", relayInfo.ID)
	if err := host.Connect(ctx, *relayInfo); err != nil {
		log.Fatalf("failed to connect to relay node %s: %v", relayInfo.ID, err)
	}
	log.Infof("connected to relay node %s", relayInfo.ID)

	for _, peerInfo := range bootstrapPeers {
		log.Infof("connecting to bootstrap node %s", peerInfo.ID)
		if err := host.Connect(ctx, peerInfo); err != nil {
			log.Errorf("Failed to connect to bootstrap node %s: %v", peerInfo.ID, err)
			continue
		}
		log.Infof("connected to bootstrap node %s", peerInfo.ID)
	}

	if kademliaDHT == nil {
		log.Fatal("dht not init properly")
	}

	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatalf("failed to bootstrap dht: %v", err)
	}

	relayaddr, err := multiaddr.NewMultiaddr("/p2p/" + relayInfo.ID.String() + "/p2p-circuit/p2p/" + host.ID().String())
	if err != nil {
		log.Error(err)
		return
	}
	nodeRelayInfo := peer.AddrInfo{
		ID:    host.ID(),
		Addrs: []multiaddr.Multiaddr{relayaddr},
	}

	fmt.Println("we are hopefully listening on following addresses")
	for _, addr := range nodeRelayInfo.Addrs {
		fmt.Printf("%s\n", addr)
	}

	time.Sleep(5 * time.Second)

	_, err = client.Reserve(context.Background(), host, *relayInfo)
	if err != nil {
		log.Info("unreachable2 failed to receive a relay reservation from relay1. %v", err)
		return
	}
	// putRelayedAddress(ctx, kademliaDHT, relayaddr)

	log.Info("announcing ourselves")
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	dutil.Advertise(ctx, routingDiscovery, rendezvousString)
	log.Info("successfully announced")

	log.Info("searching for peers")
	peerChan, err := routingDiscovery.FindPeers(ctx, rendezvousString)
	if err != nil {
		log.Fatalf("failed to find peers: %v", err)
	}

	connectedPeers := make(map[peer.ID]bool)

	for p := range peerChan {
		if p.ID == host.ID() {
			fmt.Printf("host.ID(): %v\n", host.ID())
			continue
		}
		if isBootstrapPeer(p.ID) || p.ID == relayInfo.ID {
			fmt.Printf("isBootstrapPeer(p.ID): %v\n", isBootstrapPeer(p.ID))
			fmt.Printf("isBootstrapPeer(p.ID): %v\n", relayInfo.ID)
			continue
		}

		if connectedPeers[p.ID] {
			fmt.Printf("connectedPeers[p.ID]: %v\n", connectedPeers[p.ID])
			continue
		}

		log.Infof("!!!!!\nfound peer:\n%v", p)

		// Try directly
		err := host.Connect(ctx, p)
		if err != nil {
			log.Warningf("Failed to connect to peer directly %s : %v", p.ID, err)
			// Try to connect via relay
			relayAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s/p2p-circuit/p2p/%s", relayInfo.ID, p.ID))
			if err != nil {
				log.Errorf("failed to construct relay address: %v", err)
				continue
			}
			p.Addrs = []multiaddr.Multiaddr{relayAddr}
			log.Infof("trying to connect to peer %s via relay", p.ID)
			log.Infof("addr is %s", p.Addrs)

			if err := host.Connect(ctx, p); err != nil {
				log.Warningf("failed to connect to peer %s via relay: %v", p.ID, err)
				continue
			}
			// if err := connectRelayedAddress(ctx, host, kademliaDHT, p.ID); err != nil {
			// 	log.Warningf("failed to connect to peer %s via relay: %v", p.ID, err)
			// 	continue
			// }
		}

		stream, err := host.NewStream(ctx, p.ID, protocol.ID(rendezvousString))
		if err != nil {
			log.Warningf("failed to open stream to peer %s: %v", p.ID, err)
			continue
		}

		log.Infof("connected to %s", p.ID)

		stream.Close()

		connectedPeers[p.ID] = true
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			peers := host.Network().Peers()
			if len(peers) == 0 {
				log.Warn("no peers to ping")
				continue
			}

			for _, peerID := range peers {
				if peerID == host.ID() || isBootstrapPeer(peerID) || peerID == relayInfo.ID {
					continue
				}

				go pingPeer(ctx, host, peerID, rendezvousString, log, connectedPeers)
			}
		}
	}()

	go func() {
		for {
			kademliaDHT.RefreshRoutingTable() //has a channel to block, but unused for now
			peers := kademliaDHT.RoutingTable().ListPeers()
			log.Infof("Routing table peers (%d): %v", len(peers), peers)
			time.Sleep(5 * time.Second)
		}
	}()

	select {}
}
