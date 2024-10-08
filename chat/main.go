package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
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

	addrStr := "/ip4/172.18.0.2/tcp/1237/p2p/12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n"
	// addrStr := "/dns4/bootstrap1/tcp/1237/p2p/12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n"

	// Parse the bootstrap node address
	bootstrapAddr, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		log.Fatal("Invalid bootstrap address:", err)
	}
	peerInfo, err := peer.AddrInfoFromP2pAddr(bootstrapAddr)
	if err != nil {
		log.Fatal("Failed to parse bootstrap peer info:", err)
	}
	bootstrapPeers := []peer.AddrInfo{*peerInfo}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var kademliaDHT *dht.IpfsDHT
	host, err := libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", (rand.IntN(1000)+8000))),
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays(bootstrapPeers),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
		libp2p.ForceReachabilityPrivate(),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			kademliaDHT, err = dht.New(ctx, h, dht.Mode(dht.ModeServer))
			return kademliaDHT, err
		}),
	)
	if err != nil {
		log.Fatal("Failed to create libp2p host:", err)
	}

	log.Infof("Host created. We are: %s", host.ID())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Infof("Received signal %s, shutting down...", sig)
		if err := host.Close(); err != nil {
			log.Errorf("Error closing host: %v", err)
		}
		os.Exit(0)
	}()

	rendezvousString := "meetme"
	host.SetStreamHandler(protocol.ID(rendezvousString), handleStream)

	log.Infof("Parsed Bootstrap PeerInfo: %+v", peerInfo)

	if err := host.Connect(ctx, *peerInfo); err != nil {
		log.Fatal("Failed to connect to bootstrap node:", err)
	}
	log.Infof("Connected to bootstrap node: %s", peerInfo.ID)

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
		// Skip self
		if p.ID == host.ID() {
			continue
		}
		isBootstrap := false
		for _, bootstrapID := range bootstrapPeerIDs {
			if p.ID == bootstrapID {
				isBootstrap = true
				break
			}
		}
		if isBootstrap {
			continue
		}

		// Skip if already connected
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

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			peers := host.Network().Peers()
			if len(peers) == 0 {
				log.Warnf("No peers to ping")
				continue
			}

			for _, peerID := range peers {
				if peerID == host.ID() {
					continue
				}
				isBootstrap := false
				for _, bootstrapID := range bootstrapPeerIDs {
					if peerID == bootstrapID {
						isBootstrap = true
						break
					}
				}
				if isBootstrap {
					continue
				}

				go func(pid peer.ID) {
					stream, err := host.NewStream(ctx, pid, protocol.ID(rendezvousString))
					if err != nil {
						log.Errorf("Failed to open stream to %s: %v", pid, err)
						return
					}
					defer stream.Close()

					// Send "PING\n"
					_, err = fmt.Fprintf(stream, "PING\n")
					if err != nil {
						log.Errorf("Failed to send PING to %s: %v", pid, err)
						return
					}
					log.Infof("Sent PING to %s", pid)

					// Read "PONG\n" response
					buf := make([]byte, 5) // Expecting "PONG\n"
					_, err = stream.Read(buf)
					if err != nil {
						log.Errorf("Failed to read PONG from %s: %v", pid, err)
						return
					}

					response := string(buf)
					if response == "PONG\n" {
						log.Infof("Received PONG from %s", pid)
					} else {
						log.Warnf("Unexpected response from %s: %s", pid, response)
					}
				}(peerID)
			}
		}
	}()

	select {}
}
