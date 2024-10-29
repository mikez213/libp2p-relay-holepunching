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
	"github.com/libp2p/go-libp2p/core/protocol"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"

	routing "github.com/libp2p/go-libp2p/core/routing"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("mobileclient")

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

var NodeRunnerProtocol = protocol.ID("/customprotocol/request-node-runner/1.0.0")

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
	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("mobileclient", "debug")
}

func isBootstrapPeer(peerID peer.ID) bool {
	for _, bootstrapID := range bootstrapPeerIDs {
		if peerID == bootstrapID {
			return true
		}
	}
	return false
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

func parseCmdArgs() (string, int, string, []string) {
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

	return relayAddrStr, keyIndexInt, "/customprotocol/request-node-runner/1.0.0", bootstrapAddrs
}

func getLibp2pIdentity(keyIndex int) libp2p.Option {
	clientOpt, err := RelayIdentity(keyIndex)
	if err != nil {
		log.Fatalf("client identity error: %v", err)
	}
	return clientOpt
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

	log.Info("Relay info: ", relayInfo.ID, " address", relayInfo.Addrs)

	return relayInfo
}

func createHost(ctx context.Context, clientOpt libp2p.Option, relayInfo *peer.AddrInfo) (host.Host, *dht.IpfsDHT) {
	var kademliaDHT *dht.IpfsDHT

	ListenAddrs := func(cfg *config.Config) error {
		addrs := []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/::/tcp/0",
		}
		listenAddrs := make([]multiaddr.Multiaddr, 0, len(addrs))

		for _, s := range addrs {
			addr, err := multiaddr.NewMultiaddr(s)
			if err != nil {
				return err
			}
			listenAddrs = append(listenAddrs, addr)
		}

		return cfg.Apply(libp2p.ListenAddrs(listenAddrs...))
	}

	host, err := libp2p.New(
		clientOpt,
		ListenAddrs,
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{*relayInfo}, autorelay.WithMetricsTracer(autorelay.NewMetricsTracer())),
		libp2p.NATPortMap(),
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

	log.Infof("Host created, Peer ID: %s", host.ID())

	return host, kademliaDHT
}

func connectToBootstrapPeers(ctx context.Context, host host.Host, bootstrapPeers []peer.AddrInfo) {
	for _, peerInfo := range bootstrapPeers {
		log.Infof("Connecting to bootstrap node %s", peerInfo.ID)
		if err := host.Connect(ctx, peerInfo); err != nil {
			log.Errorf("Failed to connect to bootstrap node %s: %v", peerInfo.ID, err)
			continue
		}
		log.Infof("Connected to bootstrap node %s", peerInfo.ID)
	}
}

func bootstrapDHT(ctx context.Context, kademliaDHT *dht.IpfsDHT) {
	if kademliaDHT == nil {
		log.Fatal("DHT not initialized properly")
	}

	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatalf("Failed to bootstrap DHT: %v", err)
	}
}

func connectToRelay(ctx context.Context, host host.Host, relayInfo *peer.AddrInfo) {
	log.Infof("Connecting to relay node %s", relayInfo.ID)
	if err := host.Connect(ctx, *relayInfo); err != nil {
		log.Fatalf("Failed to connect to relay node %s: %v", relayInfo.ID, err)
	}
	log.Infof("Connected to relay node %s", relayInfo.ID)
}

func reserveRelay(ctx context.Context, host host.Host, relayInfo *peer.AddrInfo) {
	for {
		log.Info("Attempting to reserve relay slot")
		_, err := client.Reserve(ctx, host, *relayInfo)
		if err != nil {
			log.Errorf("Failed to receive a relay reservation: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Info("Relay reservation successful")
		break
	}
}

func announceSelf(ctx context.Context, kademliaDHT *dht.IpfsDHT, rend string) {
	log.Info("Announcing ourselves")
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	dutil.Advertise(ctx, routingDiscovery, rend)
	log.Info("Successfully announced")
}

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

func requestNodeRunnerID(ctx context.Context, host host.Host, relayInfo *peer.AddrInfo) (peer.ID, error) {
	stream, err := host.NewStream(ctx, relayInfo.ID, NodeRunnerProtocol)
	if err != nil {
		return "", fmt.Errorf("failed to open stream to relay: %w", err)
	}
	defer stream.Close()

	_, err = fmt.Fprintf(stream, "SEND_NODE_RUNNER\n")
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}

	buf := make([]byte, 128)
	n, err := stream.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	nodeRunnerIDStr := strings.TrimSpace(string(buf[:n]))
	nodeRunnerID, err := peer.Decode(nodeRunnerIDStr)
	if err != nil {
		return "", fmt.Errorf("invalid Peer ID received: %w", err)
	}

	log.Infof("received Node Runner Peer ID: %s", nodeRunnerID)
	return nodeRunnerID, nil
}

func communicateWithNodeRunner(ctx context.Context, host host.Host, nodeRunnerID peer.ID, protocolID string) {
	stream, err := host.NewStream(ctx, nodeRunnerID, protocol.ID(protocolID))
	if err != nil {
		log.Fatalf("Failed to open stream to Node Runner: %v", err)
	}
	defer stream.Close()

	log.Info("Receiving message from Node Runner")
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil && err != io.EOF {
		log.Errorf("Error reading from stream: %v", err)
	}
	log.Infof("Received message: %s", string(buf[:n]))
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relayAddrStr, keyIndexInt, protocolID, bootstrapAddrs := parseCmdArgs()

	clientOpt := getLibp2pIdentity(keyIndexInt)

	relayInfo := parseRelayAddress(relayAddrStr)

	host, kademliaDHT := createHost(ctx, clientOpt, relayInfo)

	connectToBootstrapPeers(ctx, host, parseBootstrap(bootstrapAddrs))
	bootstrapDHT(ctx, kademliaDHT)
	connectToRelay(ctx, host, relayInfo)

	reserveRelay(ctx, host, relayInfo)
	announceSelf(ctx, kademliaDHT, protocolID)
	setupDHTRefresh(kademliaDHT)

	nodeRunnerID, err := requestNodeRunnerID(ctx, host, relayInfo)
	if err != nil {
		log.Fatalf("Failed to request Node Runner ID: %v", err)
	}

	communicateWithNodeRunner(ctx, host, nodeRunnerID, protocolID)

	select {}
}
