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

var log = logging.Logger("relaylog")

// relayer keys
var relayerPrivateKeys = []string{
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

func main() {
	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("bootstrap", "debug")

	listenPort := flag.Int("port", 1237, "TCP port to listen on")
	bootstrapPeers := flag.String("bootstrap", "", "Comma separated bootstrap peer multiaddrs")
	keyIndex := flag.Int("key", 0, "Relayer private key index")
	flag.Parse()

	relayOpt, err := RelayIdentity(*keyIndex)
	if err != nil {
		log.Fatalf("relay id err: %v", err)
	}

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

	log.Infof("bootstrap up pid %s", host.ID())
	log.Info("listening on:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}

	kademliaDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		log.Fatal(err)
	}

	log.Debug("bootstrapping dht")
	if err := kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}

	if *bootstrapPeers != "" {
		peerAddrs := strings.Split(*bootstrapPeers, ",")
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

	time.Sleep(2 * time.Second)

	log.Infof("running pid %s", host.ID())
	log.Info("use multiaddrs to connect:")
	for _, addr := range host.Addrs() {
		log.Infof("%s/p2p/%s", addr, host.ID())
	}

	go func() {
		for {
			peers := kademliaDHT.RoutingTable().ListPeers()
			log.Infof("routing table peers (%d): %v", len(peers), peers)
			time.Sleep(10 * time.Second)
		}
	}()

	select {}
}
