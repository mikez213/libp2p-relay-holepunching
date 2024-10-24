package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"

	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	multiaddr "github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("relaylog")

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

func initializeLogger() {
	logging.SetAllLoggers(logging.LevelInfo)
	// logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("relaylog", "debug")
}

func parseCommandLineArgs() (int, string, int) {
	listenPort := flag.Int("port", 1237, "TCP port to listen on")
	bootstrapPeers := flag.String("bootstrap", "", "Comma separated bootstrap peer multiaddrs")
	keyIndex := flag.Int("key", 3, "Relayer private key index") //relay keys start at 3
	flag.Parse()

	return *listenPort, *bootstrapPeers, *keyIndex
}

func getRelayIdentity(keyIndex int) libp2p.Option {
	relayOpt, err := RelayIdentity(keyIndex)
	if err != nil {
		log.Fatalf("relay identity error: %v", err)
	}
	return relayOpt
}

func createHost(ctx context.Context, relayOpt libp2p.Option, listenPort int) host.Host {
	ListenAddrs := func(cfg *config.Config) error {
		addrs := []string{
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort),
			// "/ip4/0.0.0.0/tcp/9001",
			// "/ip4/172.17.0.2/tcp/38043",
			// "/ip4/172.20.0.2/tcp/38043",
			// "/ip4/0.0.0.0/tcp/0",
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
		relayOpt,
		ListenAddrs,
		// libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)),
		libp2p.EnableRelay(),
		libp2p.EnableRelayService(),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
		libp2p.ForceReachabilityPublic(),
	)
	if err != nil {
		log.Fatal(err)
	}
	return host
}

func setupRelayService(host host.Host) {
	mt := relay.NewMetricsTracer()
	_, err := relay.New(host, relay.WithInfiniteLimits(), relay.WithMetricsTracer(mt))
	if err != nil {
		log.Infof("Failed to instantiate the relay: %v", err)
		return
	}
}

func logHostInfo(host host.Host) {
	log.Infof("relay node is running Peer ID: %s", host.ID())
	log.Info("Listening on:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}
}

func createDHT(ctx context.Context, host host.Host) *dht.IpfsDHT {
	kademliaDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		log.Fatal(err)
	}
	return kademliaDHT
}

func bootstrapDHT(ctx context.Context, kademliaDHT *dht.IpfsDHT) {
	log.Info("bootstrapping dht")
	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}
}

func connectToBootstrapPeers(ctx context.Context, host host.Host, bootstrapPeers string) {
	if bootstrapPeers != "" {
		peerAddrs := strings.Split(bootstrapPeers, ",")
		for _, addr := range peerAddrs {
			addr = strings.TrimSpace(addr)
			if addr == "" {
				log.Warn("empty bootstrap addr")
				continue
			}

			maddr, err := multiaddr.NewMultiaddr(addr)
			if err != nil {
				log.Errorf("invalid bootstrap addr '%s': %v", addr, err)
				continue
			}

			peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
			if err != nil {
				log.Errorf("get peer info failed for '%s': %v", addr, err)
				continue
			}

			host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)

			if err := host.Connect(ctx, *peerInfo); err != nil {
				log.Errorf("connect to bootstrap peer %s failed: %v", peerInfo.ID, err)
				continue
			}
			log.Infof("connected to bootstrap peer %s", peerInfo.ID)
		}
	}
}

func setupDHTRefresh(kademliaDHT *dht.IpfsDHT) {
	go func() {
		for {
			time.Sleep(10 * time.Second)

			kademliaDHT.RefreshRoutingTable()
			peers := kademliaDHT.RoutingTable().ListPeers()
			log.Infof("Routing table peers (%d): %v", len(peers), peers)
			// log.Infof("Routing table peers (%d): %v", mt.RelayStatus())

		}
	}()
}

func main() {
	initializeLogger()

	listenPort, bootstrapPeers, keyIndex := parseCommandLineArgs()

	relayOpt := getRelayIdentity(keyIndex)

	ctx := context.Background()

	host := createHost(ctx, relayOpt, listenPort)

	setupRelayService(host)

	logHostInfo(host)

	kademliaDHT := createDHT(ctx, host)

	bootstrapDHT(ctx, kademliaDHT)

	connectToBootstrapPeers(ctx, host, bootstrapPeers)

	time.Sleep(5 * time.Second)

	logHostInfo(host)

	setupDHTRefresh(kademliaDHT)

	select {}
}
