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
)

var log = logging.Logger("standardnode")

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
			log.Fatalf("Failed to decode bootstrap Peer ID '%s': %v", idStr, err)
		}
		bootstrapPeerIDs = append(bootstrapPeerIDs, pid)
	}
}

// isBootstrapPeer checks if the given peer ID is one of the bootstrap peers.
func isBootstrapPeer(peerID peer.ID) bool {
	for _, bootstrapID := range bootstrapPeerIDs {
		if peerID == bootstrapID {
			return true
		}
	}
	return false
}

// pingPeer handles the ping-pong interaction with a peer.
func pingPeer(ctx context.Context, host host.Host, pid peer.ID, rendezvous string, log logging.EventLogger, connectedPeers map[peer.ID]bool) {
	stream, err := host.NewStream(ctx, pid, protocol.ID(rendezvous))
	if err != nil {
		log.Errorf("Failed to open stream to %s: %v", pid, err)
		return
	}
	defer stream.Close()

	// Send "PING\n"
	if _, err := fmt.Fprintf(stream, "PING\n"); err != nil {
		log.Errorf("Failed to send PING to %s: %v", pid, err)
		return
	}
	log.Infof("Sent PING to %s", pid)

	// Read "PONG\n" response
	buf := make([]byte, 5) // Expecting "PONG\n"
	if _, err := stream.Read(buf); err != nil {
		log.Errorf("Failed to read PONG from %s: %v", pid, err)
		return
	}

	if string(buf) == "PONG\n" {
		log.Infof("Received PONG from %s", pid)
		connectedPeers[pid] = true
	} else {
		log.Warnf("Unexpected response from %s: %s", pid, string(buf))
	}
}

func handleStream(stream network.Stream) {
	log.Info("Received a new stream from", stream.Conn().RemotePeer())

	defer stream.Close()

	buf := make([]byte, 5) // Expecting "PING\n"
	_, err := stream.Read(buf)
	if err != nil {
		log.Error("Error reading from stream:", err)
		return
	}

	received := string(buf)
	log.Infof("Received message: %s from %s", received, stream.Conn().RemotePeer())

	if received == "PING\n" {
		// Respond with "PONG\n"
		_, err = fmt.Fprintf(stream, "PONG\n")
		if err != nil {
			log.Error("Error writing PONG to stream:", err)
			return
		}
		log.Infof("Sent PONG to %s", stream.Conn().RemotePeer())
	} else {
		log.Infof("Unexpected message: %s from %s", received, stream.Conn().RemotePeer())
	}
}

func main() {
	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("standardnode", "debug")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parse command-line arguments for bootstrap addresses
	if len(os.Args) < 2 {
		log.Fatal("Please provide at least one bootstrap node address as a command-line argument")
	}

	bootstrapAddrs := os.Args[1:]

	// Convert bootstrap addresses to AddrInfo
	var bootstrapPeers []peer.AddrInfo
	for _, addrStr := range bootstrapAddrs {
		addrStr = strings.TrimSpace(addrStr)
		if addrStr == "" {
			continue
		}
		maddr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			log.Errorf("Invalid bootstrap address '%s': %v", addrStr, err)
			continue
		}
		peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Errorf("Failed to parse bootstrap peer info from '%s': %v", addrStr, err)
			continue
		}
		bootstrapPeers = append(bootstrapPeers, *peerInfo)
	}

	if len(bootstrapPeers) == 0 {
		log.Fatal("No valid bootstrap addresses provided")
	}

	var kademliaDHT *dht.IpfsDHT
	host, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.EnableRelay(),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			kademliaDHT, _ = dht.New(ctx, h, dht.Mode(dht.ModeServer))
			return kademliaDHT, nil
		}),
	)
	if err != nil {
		log.Fatal("Failed to create libp2p host:", err)
	}

	log.Infof("Host created. We are: %s", host.ID())

	rendezvousString := "meetme"
	host.SetStreamHandler(protocol.ID(rendezvousString), handleStream)

	// Connect to all bootstrap nodes
	for _, peerInfo := range bootstrapPeers {
		log.Infof("Connecting to bootstrap node: %s", peerInfo.ID)
		if err := host.Connect(ctx, peerInfo); err != nil {
			log.Errorf("Failed to connect to bootstrap node %s: %v", peerInfo.ID, err)
			continue
		}
		log.Infof("Connected to bootstrap node: %s", peerInfo.ID)
	}

	if kademliaDHT == nil {
		log.Fatal("DHT was not initialized properly.")
	}

	time.Sleep(5 * time.Second)

	log.Info("Announcing ourselves...")
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	dutil.Advertise(ctx, routingDiscovery, rendezvousString)
	log.Debug("Successfully announced!")

	log.Debug("Searching for other peers...")
	peerChan, err := routingDiscovery.FindPeers(ctx, rendezvousString)
	if err != nil {
		log.Fatal("Failed to find peers:", err)
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

		log.Info("Found peer:", p)

		stream, err := host.NewStream(ctx, p.ID, protocol.ID(rendezvousString))
		if err != nil {
			log.Warning("Connection failed:", err)
			continue
		}

		log.Infof("Connected to: %s", p.ID)

		stream.Close()

		connectedPeers[p.ID] = true
	}

	// Simplified Ping Section without connectedPeers condition
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			peers := host.Network().Peers()
			if len(peers) == 0 {
				log.Warn("No peers to ping")
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
