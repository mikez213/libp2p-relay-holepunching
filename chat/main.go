package main

import (
	"bufio"
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"
	routing "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	util "github.com/libp2p/go-libp2p/p2p/discovery/util"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("standardnode")

// const BootstrapAddr = "/ip4/10.0.20.118/tcp/1237/p2p/12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n"

func handleStream(stream network.Stream) {
	log.Info("Got a new stream\n!!!!!\n")

	fmt.Printf(stream.ID(), stream.Stat())
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go readData(rw)
	go writeData(rw)

}

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			log.Error("Error reading from buffer:", err)
			return
		}

		if str != "" && str != "\n" {
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}
	}
}

func writeData(rw *bufio.ReadWriter) {
	stdReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			log.Error("Error reading from stdin:", err)
			return
		}

		_, err = rw.WriteString(sendData)
		if err != nil {
			log.Error("Error writing to buffer:", err)
			return
		}
		err = rw.Flush()
		if err != nil {
			log.Error("Error flushing buffer:", err)
			return
		}
	}
}

func main() {
	// Initialize logging
	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("standardnode", "debug")

	addrStr := "/ip4/10.0.20.118/tcp/1237/p2p/12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n"

	// addrStr := "/ip4/127.0.0.1/tcp/12354"
	// Parse the bootstrap node address
	bootstrapAddr, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		log.Fatal(err)
	}
	peerinfo, err := peer.AddrInfoFromP2pAddr(bootstrapAddr)
	if err != nil {
		log.Fatal(err)
	}
	bootstrapPeers := []peer.AddrInfo{*peerinfo}

	host, err := libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", (rand.IntN(1000)+8000))),
		// libp2p.NoListenAddrs,

		libp2p.EnableRelay(),
		// libp2p.EnableAutoRelay(),
		libp2p.EnableAutoRelayWithStaticRelays(bootstrapPeers),
		// libp2p.EnableAutoRelayWithPeerSource(),

		// libp2p.RelayOptions(libp2p.EnableHop),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
		libp2p.ForceReachabilityPrivate(),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Host created. We are: %s", host.ID())

	// Set stream handler
	rendezvousString := "meetme"
	host.SetStreamHandler(protocol.ID(rendezvousString), handleStream)

	log.Infof("Parsed Bootstrap PeerInfo: %+v", peerinfo)

	// Connect to the bootstrap node
	if err := host.Connect(context.Background(), *peerinfo); err != nil {
		log.Fatal(err)
	}
	log.Infof("Connected to bootstrap node: %s", peerinfo.ID)

	// Start DHT
	kademliaDHT, err := dht.New(context.Background(), host)
	if err != nil {
		log.Fatal(err)
	}

	// Bootstrap the DHT
	log.Debug("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Wait for the DHT to bootstrap
	time.Sleep(5 * time.Second)

	// Announce ourselves
	log.Info("Announcing ourselves...")
	routingDiscovery := routing.NewRoutingDiscovery(kademliaDHT)
	util.Advertise(context.Background(), routingDiscovery, rendezvousString)
	log.Debug("Successfully announced!")

	// Discover peers
	log.Debug("Searching for other peers...")
	peerChan, err := routingDiscovery.FindPeers(context.Background(), rendezvousString)
	if err != nil {
		log.Fatal(err)
	}

	for peer := range peerChan {
		if peer.ID == host.ID() || peer.ID == peerinfo.ID {
			continue
		}

		log.Debug("Found peer:", peer)

		log.Debug("Connecting to:", peer)
		stream, err := host.NewStream(context.Background(), peer.ID, protocol.ID(rendezvousString))
		if err != nil {
			log.Warning("Connection failed:", err)
			continue
		}

		log.Infof("Connected to: %s\n!!!!!\n", peer)

		rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

		go writeData(rw)
		go readData(rw)
	}

	select {}
}
