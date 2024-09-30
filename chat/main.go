package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"

	// discovery "github.com/libp2p/go-libp2p/p2p/discovery"
	routing "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	util "github.com/libp2p/go-libp2p/p2p/discovery/util"

	"github.com/libp2p/go-libp2p/core/network"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("standardnode")

func handleStream(stream network.Stream) {
	log.Info("Got a new stream!")

	// Create a buffer stream for non-blocking read and write.
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go readData(rw)
	go writeData(rw)

	// 'stream' will stay open until you close it (or the other side closes it).
}

func readData(rw *bufio.ReadWriter) {
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			log.Error("Error reading from buffer: ", err)
			return
		}

		if str == "" {
			return
		}
		if str != "\n" {
			// Green console color: 	\x1b[32m
			// Reset console color: 	\x1b[0m
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
			log.Error("Error reading from stdin: ", err)
			return
		}

		_, err = rw.WriteString(fmt.Sprintf("%s\n", sendData))
		if err != nil {
			log.Error("Error writing to buffer: ", err)
			return
		}
		err = rw.Flush()
		if err != nil {
			log.Error("Error flushing buffer: ", err)
			return
		}
	}
}

func main() {
	// Initialize logging
	logging.SetAllLoggers(logging.LevelInfo)
	logging.SetLogLevel("standardnode", "debug")

	// Parse command-line flags
	// listenPort := flag.Int("port", 0, "Port to listen on")
	// bootstrapAddrStr := flag.String("bootstrap-peer", "", "Bootstrap node multiaddress")
	rendezvousString := flag.String("rendezvous", "meetme", "Unique string for rendezvous point")
	flag.Parse()

	// if *listenPort == 0 {
	// 	fmt.Println("Please provide a port to bind on with -port")
	// 	os.Exit(1)
	// }
	// if *bootstrapAddrStr == "" {
	// 	fmt.Println("Please provide the bootstrap node address with -bootstrap-peer")
	// 	os.Exit(1)
	// }
	bootstrapAddrStr := "/ip4/10.0.20.118/tcp/1237/p2p/12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n"
	// bootstrapAddrStr := os.Getenv("BOOTSTRAP_ADDR")
	if bootstrapAddrStr == "" {
		log.Fatal("BOOTSTRAP_ADDR env not set")
	}

	// Create a new libp2p Host
	ctx := context.Background()
	host, err := libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", (rand.IntN(1000)+8000))),
		// libp2p.NoListenAddrs,

		libp2p.EnableRelay(),
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Host created. We are:", host.ID())
	log.Info(host.Addrs())

	// Set stream handler
	host.SetStreamHandler(protocol.ID(*rendezvousString), handleStream)

	// Parse the bootstrap node address
	bootstrapAddr, err := multiaddr.NewMultiaddr(bootstrapAddrStr)
	if err != nil {
		log.Fatal(err)
	}
	peerinfo, err := peer.AddrInfoFromP2pAddr(bootstrapAddr)
	if err != nil {
		log.Fatal(err)
	}

	// Start DHT with the bootstrap node as a bootstrap peer
	kademliaDHT, err := dht.New(ctx, host, dht.BootstrapPeers(*peerinfo))
	if err != nil {
		log.Fatal(err)
	}

	// Bootstrap the DHT
	log.Debug("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}

	// Wait for the DHT to bootstrap
	time.Sleep(5 * time.Second)

	// Announce ourselves
	log.Info("Announcing ourselves...")
	routingDiscovery := routing.NewRoutingDiscovery(kademliaDHT)
	util.Advertise(ctx, routingDiscovery, *rendezvousString)
	log.Debug("Successfully announced!")

	// Discover peers
	log.Debug("Searching for other peers...")
	peerChan, err := routingDiscovery.FindPeers(ctx, *rendezvousString)
	if err != nil {
		log.Fatal(err)
	}

	for peer := range peerChan {
		if peer.ID == host.ID() {
			continue
		}

		log.Debug("Found peer:", peer)

		log.Debug("Connecting to:", peer)
		stream, err := host.NewStream(ctx, peer.ID, protocol.ID(*rendezvousString))

		if err != nil {
			log.Warning("Connection failed:", err)
			continue
		}

		log.Info("Connected to:", peer, "!!!!!!!!\n!!!!!!!\n\n\n\n\n!!!!!!!")

		rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

		go writeData(rw)
		go readData(rw)
	}

	// Keep the process running
	select {}
}
