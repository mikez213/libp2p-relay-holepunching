package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"

	routing "github.com/libp2p/go-libp2p/core/routing"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"

	// "github.com/libp2p/go-libp2p/p2p/host/autonat"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"

	// "github.com/libp2p/go-libp2p/p2p/protocol/holepunch"

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

	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("chatlog", "debug")
}

func isBootstrapPeer(peerID peer.ID) bool {
	for _, bootstrapID := range bootstrapPeerIDs {
		if peerID == bootstrapID {
			return true
		}
	}
	return false
}

func pingPeer(ctx context.Context, host host.Host, pid peer.ID, rend string, connectedPeers map[peer.ID]bool) {
	log.Infof("attempting to open ping stream to %s", pid)

	stream, err := host.NewStream(ctx, pid, protocol.ID(rend))
	if err != nil {
		log.Errorf("failed to open ping stream to %s: %v", pid, err)
		return
	}
	log.Infof("ping stream to %s opened successfully", pid)

	defer stream.Close()

	// Send PING message
	if _, err := fmt.Fprintf(stream, "PING\n"); err != nil {
		log.Errorf("failed to send ping to %s: %v", pid, err)
		return
	}
	log.Infof("PING message sent to %s", pid)

	// Read PONG response
	buf := make([]byte, 5)
	n, err := stream.Read(buf)
	if err != nil {
		if err == io.EOF {
			log.Infof("stream closed by peer %s after sending EOF", pid)
		} else {
			log.Errorf("failed to read PONG from %s: %v", pid, err)
		}
		return
	}

	response := string(buf[:n])
	log.Infof("Received message from %s: %s", pid, response)

	// Check if the message is PONG
	if response == "PONG\n" {
		log.Infof("Received valid PONG from %s", pid)
		connectedPeers[pid] = true
	} else {
		log.Warnf("Unexpected response from %s: str'%s' byte'%08b'", pid, response, response)
	}
}

func handleStream(stream network.Stream) {
	peerID := stream.Conn().RemotePeer()
	log.Infof("got new stream from %s", peerID)

	defer stream.Close()

	buf := make([]byte, 5)
	maxRetry := 5

	for i := 0; i < maxRetry; i++ {
		log.Infof("attempting to read from stream, attempt %d", i)
		n, err := stream.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Warnf("received EOF from %s, retrying read attempt %d/%d", peerID, i, maxRetry)
				time.Sleep(1 * time.Second)
				continue
			} else {
				log.Errorf("error reading from stream: %v", err)
				return
			}
		}

		received := string(buf[:n])
		log.Infof("received message: %s from %s", received, peerID)

		if received == "PING\n" {
			log.Infof("received PING from %s, responding with PONG", peerID)
			if _, err = fmt.Fprintf(stream, "PONG\n"); err != nil {
				log.Errorf("error writing PONG to stream: %v", err)
				return
			}
			log.Infof("sent PONG to %s", peerID)
		} else {
			log.Warnf("unexpected message from %s: %s", peerID, received)
		}

		break
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

func parseCmdArgs() (string, int, []string) {
	if len(os.Args) < 3 {
		log.Fatal("need a bootstrap node and relay")
	}

	relayAddrStr := os.Args[1]
	keyIndexStr := os.Args[2]
	bootstrapAddrs := os.Args[3:]

	keyIndexInt, err := strconv.Atoi(keyIndexStr)
	if err != nil {
		log.Fatalf("index error: %v", err)
	}

	return relayAddrStr, keyIndexInt, bootstrapAddrs
}

func getLibp2pIdentity(keyIndex int) libp2p.Option {
	nodeOpt, err := RelayIdentity(keyIndex)
	if err != nil {
		log.Fatalf("relay identity error: %v", err)
	}
	return nodeOpt
}

func parseRelayAddress(relayAddrStr string) *peer.AddrInfo {
	relayMaddr, err := multiaddr.NewMultiaddr(relayAddrStr)
	if err != nil {
		log.Fatalf("bad relay address '%s': %v", relayAddrStr, err)
	}

	relayInfo, err := peer.AddrInfoFromP2pAddr(relayMaddr)
	if err != nil {
		log.Fatalf("fail to parse relay peer info from '%s': %v", relayAddrStr, err)
	}

	log.Info("relay info: ", relayInfo.ID, " address", relayInfo.Addrs)

	return relayInfo
}

func createHost(ctx context.Context, nodeOpt libp2p.Option, relayInfo *peer.AddrInfo) (host.Host, *dht.IpfsDHT) {
	mt := autorelay.NewMetricsTracer()
	var kademliaDHT *dht.IpfsDHT

	ListenAddrs := func(cfg *config.Config) error {
		addrs := []string{
			// fmt.Sprintf("/ip4/0.0.0.0/tcp/", listenPort),
			// "/ip4/0.0.0.0/tcp/9001",
			// "/ip4/172.17.0.2/tcp/38043",
			// "/ip4/172.20.0.2/tcp/38043",
			"/ip4/0.0.0.0/tcp/0",
			// "/ip4/0.0.0.0/udp/0/quic-v1",
			// "/ip4/0.0.0.0/udp/0/quic-v1/webtransport",
			// "/ip4/0.0.0.0/udp/0/webrtc-direct",
			"/ip6/::/tcp/0",
			// "/ip6/::/udp/0/quic-v1",
			// "/ip6/::/udp/0/quic-v1/webtransport",
			// "/ip6/::/udp/0/webrtc-direct",
		}
		listenAddrs := make([]multiaddr.Multiaddr, 0, len(addrs))

		log.Info(" * Addresses %v", listenAddrs)

		for _, s := range addrs {
			addr, err := multiaddr.NewMultiaddr(s)
			if err != nil {
				return err
			}
			listenAddrs = append(listenAddrs, addr)
		}

		log.Info(" * Addresses %v", listenAddrs)

		return cfg.Apply(libp2p.ListenAddrs(listenAddrs...))
	}

	host, err := libp2p.New(
		nodeOpt,
		ListenAddrs,
		// libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{*relayInfo}, autorelay.WithMetricsTracer(mt)),
		libp2p.NATPortMap(),
		// libp2p.EnableRelayService(), // we are not a relay server
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.ForceReachabilityPrivate(),
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

	return host, kademliaDHT
}

func setupStreamHandler(host host.Host, rend string) {
	host.SetStreamHandler(protocol.ID(rend), handleStream)
}

func connectToBootstrapPeers(ctx context.Context, host host.Host, bootstrapPeers []peer.AddrInfo) {
	for _, peerInfo := range bootstrapPeers {
		log.Infof("connecting to bootstrap node %s", peerInfo.ID)
		if err := host.Connect(ctx, peerInfo); err != nil {
			log.Errorf("Failed to connect to bootstrap node %s: %v", peerInfo.ID, err)
			continue
		}
		log.Infof("connected to bootstrap node %s", peerInfo.ID)
	}
}

func bootstrapDHT(ctx context.Context, kademliaDHT *dht.IpfsDHT) {
	if kademliaDHT == nil {
		log.Fatal("dht not init properly")
	}

	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatalf("failed to bootstrap dht: %v", err)
	}
}

func connectToRelay(ctx context.Context, host host.Host, relayInfo *peer.AddrInfo) {
	log.Infof("connecting to relay node %s", relayInfo.ID)
	if err := host.Connect(ctx, *relayInfo); err != nil {
		log.Fatalf("failed to connect to relay node %s: %v", relayInfo.ID, err)
	}
	log.Infof("connected to relay node %s", relayInfo.ID)
}

func constructRelayAddresses(host host.Host, relayInfo *peer.AddrInfo) []peer.AddrInfo {

	var relayAddresses []peer.AddrInfo

	for _, addr := range relayInfo.Addrs {
		fullRelayAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p-circuit/p2p/%s", relayInfo.ID))
		if err != nil {
			log.Errorf("failed to create relay circuit multiaddr: %v", err)
			continue
		}
		log.Infof("created relay circuit multiaddr: %s", fullRelayAddr)

		combinedAddr := addr.Encapsulate(fullRelayAddr)
		log.Infof("final addr: %s", combinedAddr)

		relayAddrInfo := peer.AddrInfo{
			ID:    relayInfo.ID,
			Addrs: []multiaddr.Multiaddr{combinedAddr},
		}

		relayAddresses = append(relayAddresses, relayAddrInfo)
	}

	log.Infof("we are hopefully listening on following relay addresses:")
	for _, addrInfo := range relayAddresses {
		for _, addr := range addrInfo.Addrs {
			fmt.Printf("%s/p2p/%s\n", addr, host.ID())
		}
	}

	return relayAddresses
}

func reserveRelay(ctx context.Context, host host.Host, relayInfo *peer.AddrInfo) {
	_, err := client.Reserve(ctx, host, *relayInfo)
	if err != nil {
		log.Errorf("failed to receive a relay reservation from relay %v", err)
		return
	}
}

func announceSelf(ctx context.Context, kademliaDHT *dht.IpfsDHT, rend string) {
	log.Info("announcing ourselves")
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	dutil.Advertise(ctx, routingDiscovery, rend)
	log.Info("successfully announced")
}

func searchForPeers(ctx context.Context, kademliaDHT *dht.IpfsDHT, rend string) (<-chan peer.AddrInfo, error) {
	log.Info("searching for peers")
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	peerChan, err := routingDiscovery.FindPeers(ctx, rend)
	if err != nil {
		log.Fatalf("failed to find peers: %v", err)
	}
	return peerChan, nil
}

func containsPeer(relayAddresses []peer.AddrInfo, pid peer.ID) bool {
	for _, relayAddrInfo := range relayAddresses {
		if relayAddrInfo.ID == pid {
			return true
		}
	}
	return false
}

func connectToPeers(ctx context.Context, host host.Host, relayAddresses []peer.AddrInfo, pChan <-chan peer.AddrInfo, connectedPeers map[peer.ID]bool, rend string) {
	for p := range pChan {
		if p.ID == host.ID() {
			fmt.Printf("host.ID: %v\n", host.ID())
			continue
		}
		if isBootstrapPeer(p.ID) || containsPeer(relayAddresses, p.ID) {
			fmt.Printf("isBootstrapPeer: %v\n", isBootstrapPeer(p.ID))
			fmt.Printf("isRelayPeer: %v\n", containsPeer(relayAddresses, p.ID))
			continue
		}

		if connectedPeers[p.ID] {
			fmt.Printf("connectedPeers: %v\n", connectedPeers[p.ID])
			fmt.Printf("retry discovery for now, should skip later")

			// continue
		}

		log.Errorf("!!!!!\nfound peer:\n%v", p)

		err := host.Connect(ctx, p)
		if err != nil {
			log.Warningf("failed to connect to peer directly %s : %v", p.ID, err)
			for _, relayAddrInfo := range relayAddresses {
				relayAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p-circuit/p2p/%s", p.ID))
				// relayAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p-circuit/p2p/%s", relayAddrInfo.ID))

				if err != nil {
					log.Errorf("failed to create relay circuit multiaddr: %v", err)
					continue
				}

				combinedRelayAddr := relayAddrInfo.Addrs[0].Encapsulate(relayAddr)

				p.Addrs = append(p.Addrs, combinedRelayAddr)

				log.Infof("trying to connect to peer %s via relay %s", p.ID, relayAddrInfo.ID)
				log.Infof("relay address: %s", combinedRelayAddr)

				if err := host.Connect(ctx, relayAddrInfo); err != nil {
					log.Warningf("failed to connect to peer %s via relay %s: %v", p.ID, relayAddrInfo.ID, err)
					continue
				}
				log.Infof("we have connected to peer %s via relay %s", p.ID, relayAddrInfo.ID)

				log.Infof("attempting to open stream to peer %s with ID %s", p.ID, protocol.ID(rend))

				for i := 0; i < 5; i++ {
					stream, err := host.NewStream(ctx, relayAddrInfo.ID, protocol.ID(rend))
					if err != nil {
						log.Warningf("failed to open stream to peer %s: %v, attempt %d/5", p.ID, err, i)
						continue
					}
					log.Infof("succesfully streaming to %s via relay %s", p.ID, relayAddrInfo.ID)
					stream.Close()

					connectedPeers[p.ID] = true
					break
				}
				break
			}
		} else {
			stream, err := host.NewStream(ctx, p.ID, protocol.ID(rend))
			if err != nil {
				log.Warningf("failed to open stream to peer directly %s: %v", p.ID, err)
				continue
			}

			log.Infof("connected to %s", p.ID)

			stream.Close()

			connectedPeers[p.ID] = true
		}
	}
}

// this function breaks the sending
// func setupPeriodicPing(ctx context.Context, host host.Host, relayInfo *peer.AddrInfo, connectedPeers map[peer.ID]bool, rend string) {
// 	ticker := time.NewTicker(10 * time.Second)
// 	defer ticker.Stop()

// 	go func() {
// 		for range ticker.C {
// 			peers := host.Network().Peers()
// 			if len(peers) == 0 {
// 				log.Warn("no peers to ping")
// 				continue
// 			}

// 			for _, peerID := range peers {
// 				if peerID == host.ID() || isBootstrapPeer(peerID) || peerID == relayInfo.ID {
// 					continue
// 				}

// 				go pingPeer(ctx, host, peerID, rend, connectedPeers)
// 			}
// 		}
// 	}()
// }

func setupDHTRefresh(kademliaDHT *dht.IpfsDHT) {
	go func() {
		for {
			kademliaDHT.RefreshRoutingTable()
			peers := kademliaDHT.RoutingTable().ListPeers()
			log.Infof("Routing table peers (%d): %v", len(peers), peers)
			time.Sleep(30 * time.Second)
		}
	}()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relayAddrStr, keyIndexInt, bootstrapAddrs := parseCmdArgs()

	nodeOpt := getLibp2pIdentity(keyIndexInt)

	relayInfo := parseRelayAddress(relayAddrStr)

	bootstrapPeers := parseBootstrap(bootstrapAddrs)
	if len(bootstrapPeers) == 0 {
		log.Fatal("no valid bootstrap addrs")
	}

	host, kademliaDHT := createHost(ctx, nodeOpt, relayInfo)

	// rend := "meetmee"
	rend := "/ipfs/ping/1.0.0"
	// rend := "/libp2p/circuit/relay/0.2.0/hop"
	// rend := "/libp2p/dcutr"

	// log.Infof("our protocol: %s", protocol.ID(rend))
	// setupStreamHandler(host, rend)

	connectToBootstrapPeers(ctx, host, bootstrapPeers)
	bootstrapDHT(ctx, kademliaDHT)
	connectToRelay(ctx, host, relayInfo)

	log.Infof("our protocol: %s", protocol.ID(rend))
	setupStreamHandler(host, rend)

	relayAddresses := constructRelayAddresses(host, relayInfo)

	log.Infof("waiting 10 sec for stablity")
	time.Sleep(10 * time.Second)

	reserveRelay(ctx, host, relayInfo)

	announceSelf(ctx, kademliaDHT, rend)
	peerChan, err := searchForPeers(ctx, kademliaDHT, rend)
	if err != nil {
		log.Fatalf("failed to find peers: %v", err)
	}

	connectedPeers := make(map[peer.ID]bool)
	connectToPeers(ctx, host, relayAddresses, peerChan, connectedPeers, rend)

	// setupPeriodicPing(ctx, host, relayInfo, connectedPeers,  rend)

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
				if peerID == host.ID() || isBootstrapPeer(peerID) {
					continue
				}

				go pingPeer(ctx, host, peerID, rend, connectedPeers)
			}
		}
	}()

	setupDHTRefresh(kademliaDHT)

	select {}
}
