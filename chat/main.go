package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"

	routing "github.com/libp2p/go-libp2p/core/routing"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	// "github.com/libp2p/go-nat"
)

var log = logging.Logger("chatlog")

var bootstrapPeerIDs = []peer.ID{}

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

func main() {
	// logging.SetAllLoggers(logging.LevelDebug)
	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("chatnode", "debug")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if len(os.Args) < 2 {
		log.Fatal("need a bootstrap node")
	}

	bootstrapAddrs := os.Args[1:]

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

	if len(bootstrapPeers) == 0 {
		log.Fatal("no valid bootstrap addrs")
	}

	var kademliaDHT *dht.IpfsDHT
	host, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays(bootstrapPeers),
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

	log.Infof("host created, we are %s", host.ID())

	rendezvousString := "meetme"
	host.SetStreamHandler(protocol.ID(rendezvousString), handleStream)

	for _, peerInfo := range bootstrapPeers {
		log.Infof("connecting to bootstrap node %s", peerInfo.ID)
		if err := host.Connect(ctx, peerInfo); err != nil {
			log.Errorf("failed to connect to bootstrap node %s: %v", peerInfo.ID, err)
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

	time.Sleep(10 * time.Second)

	log.Info("announcing ourselves")
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	dutil.Advertise(ctx, routingDiscovery, rendezvousString)
	log.Debug("successfully announced")

	log.Debug("searching for peers")
	peerChan, err := routingDiscovery.FindPeers(ctx, rendezvousString)
	if err != nil {
		log.Fatalf("failed to find peers: %v", err)
	}

	connectedPeers := make(map[peer.ID]bool)

	for p := range peerChan {

		if p.ID == host.ID() {
			continue
		}
		if isBootstrapPeer(p.ID) {
			continue
		}

		if connectedPeers[p.ID] {
			continue
		}

		// relayAddrs := host.Network().Peerstore().Addrs(p.ID)
		// if len(relayAddrs) > 0 {
		// 	p.Addrs = append(p.Addrs, relayAddrs...)
		// }
		// if err := host.Connect(ctx, p); err != nil {
		// 	log.Warningf("failed to connect to peer %s: %v", p.ID, err)
		// 	continue
		// }

		log.Infof("found peer: %v", p)

		if err := host.Connect(ctx, p); err != nil {
			log.Warningf("failed to connect to peer %s: %v", p.ID, err)
			continue
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
				if peerID == host.ID() || isBootstrapPeer(peerID) {
					continue
				}

				go pingPeer(ctx, host, peerID, rendezvousString, log, connectedPeers)
			}
		}
	}()

	select {}
}
