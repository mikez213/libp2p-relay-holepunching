// relay.go
package main

import (
	"context"
	"flag"
	"fmt" // Added import for io
	"io"
	"strings"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"

	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"

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

	//nodes
	"CAESQFffsVM3eUXLozmXkBM2FSSVhEmo/Cq5RlXOAAaniTdCu3EQ6Zf7lQDasCj6IXyTihFQWZB+nmGFn/ZAA5y5egk=",
	//12D3KooWNS4QQxwNURwoYoXmGjH9AQkagcGTjRUQT33P4i4FKQsi
	"CAESQCSHrfyzNZkxwoNmXI1wx5Lvr6o4+kGxGepFH0AfYlKthyON+1hQRjLJQaBAQLrr1cfMHFFoC40X62DQIhL246U=",
	//12D3KooWJuteouY1d5SYFcAUAYDVPjFD8MUBgqsdjZfBkAecCS2Y
	"CAESQDyiSqC9Jez8wKSQs74YJalAegamjVKHbnaN35pfe6Gk21WVgCzfvBdLVoRj8XXny/k1LtSOhPZWNz0rWKCOYpk=",
	//12D3KooWQaZ9Ppi8A2hcEspJhewfPqKjtXu4vx7FQPaUGnHXWpNL
}

// var NodeRunnerProtocol = protocol.ID("/customprotocol/request-node-runner/1.0.0")
// var NodeRunnerProtocol = protocol.ID(identify.ID)
var NodeRunnerProtocol = protocol.ID("/customprotocol/1.0.0")

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

	// logging.SetAllLoggers(logging.LevelWarn)
	logging.SetAllLoggers(logging.LevelDebug)

	// logging.SetLogLevel("dht", "error") // get rid of  network size estimator track peers: expected bucket size number of peers

	logging.SetLogLevel("relaylog", "debug")
}

func parseCommandLineArgs() (int, string, int, string) {
	listenPort := flag.Int("port", 1237, "TCP port to listen on")
	bootstrapPeers := flag.String("bootstrap", "", "Comma separated bootstrap peer multiaddrs")
	keyIndex := flag.Int("key", 0, "Relayer private key index")
	noderunnerID := flag.String("noderunner", "", "Peer ID of the Node Runner")
	flag.Parse()

	return *listenPort, *bootstrapPeers, *keyIndex, *noderunnerID
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
		relayOpt,
		ListenAddrs,
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

func setupRelayService(host host.Host) *relay.Relay {
	mt := relay.NewMetricsTracer()
	log.Debugf("Relay timeouts: %d %d %d",
		relay.ConnectTimeout,
		relay.StreamTimeout,
		relay.HandshakeTimeout)

	relayService, err := relay.New(host, relay.WithInfiniteLimits(), relay.WithMetricsTracer(mt))
	log.Debugf("relayservice %+v", relayService)
	// mt.RelayStatus(true)
	// limit resources?
	// mt.RelayStatus(true)
	// var status pb.Status
	if err != nil {
		log.Fatalf("Failed to instantiate the relay: %v", err)
		return nil
	}
	return relayService
}

func logHostInfo(host host.Host) {
	log.Infof("Relay node is running. Peer ID: %s", host.ID())
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
	log.Info("Bootstrapping DHT")
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
			log.Infof("Connected to bootstrap peer %s", peerInfo.ID)
		}
	}
}

func setupDHTRefresh(kademliaDHT *dht.IpfsDHT) {
	go func() {
		for {
			time.Sleep(30 * time.Second)
			kademliaDHT.RefreshRoutingTable()
			peers := kademliaDHT.RoutingTable().ListPeers()
			log.Infof("Routing table peers (%d): %v", len(peers), peers)
			// log.Infof("Relay Status (%d): %v", mt.RelayStatus())
			// var status pb.Status
			// mt.ConnectionRequestHandled(status)
			// log.Info(status)
			// mt.ReservationRequestHandled(status)
			// log.Info(status)
		}
	}()
}

// func handleNodeRunnerIDRequest(stream network.Stream, nodeRunnerID peer.ID) {
// 	defer stream.Close()
// 	log.Infof("Received Node Runner ID request from %s", stream.Conn().RemotePeer())

// 	// Read the request message
// 	buf := make([]byte, 256)
// 	n, err := stream.Read(buf)
// 	if err != nil {
// 		if err != io.EOF {
// 			log.Errorf("Error reading request: %v", err)
// 		}
// 		return
// 	}

// 	request := strings.TrimSpace(string(buf[:n]))
// 	log.Infof("got request: %s", request)
// 	if request != "PING" {
// 		log.Warnf("Invalid request from %s: %s", stream.Conn().RemotePeer(), request)
// 		return
// 	}

// 	// Respond with the Node Runner's Peer ID
// 	response := fmt.Sprintf("%s\n", nodeRunnerID.String())
// 	_, err = stream.Write([]byte(response))
// 	if err != nil {
// 		log.Errorf("Error writing response to %s: %v", stream.Conn().RemotePeer(), err)
// 		return
// 	}

// 	log.Infof("Sent Node Runner Peer ID %s to %s", nodeRunnerID, stream.Conn().RemotePeer())
// }

// func requestNodeRunnerID(ctx context.Context, host host.Host, relayInfo *peer.AddrInfo) (peer.ID, error) {
// 	stream, err := host.NewStream(ctx, relayInfo.ID, NodeRunnerProtocol)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to open stream to relay: %w", err)
// 	}
// 	defer stream.Close()

// 	_, err = fmt.Fprintf(stream, "REQUEST_NODE_RUNNER_ID\n")
// 	if err != nil {
// 		return "", fmt.Errorf("failed to send request: %w", err)
// 	}

// 	buf := make([]byte, 128)
// 	n, err := stream.Read(buf)
// 	if err != nil && err != io.EOF {
// 		return "", fmt.Errorf("failed to read response: %w", err)
// 	}

// 	nodeRunnerIDStr := strings.TrimSpace(string(buf[:n]))
// 	nodeRunnerID, err := peer.Decode(nodeRunnerIDStr)
// 	if err != nil {
// 		return "", fmt.Errorf("invalid Peer ID received: %w", err)
// 	}

// 	log.Infof("Received Node Runner Peer ID: %s", nodeRunnerID)
// 	return nodeRunnerID, nil
// }

// func communicateWithNodeRunner(ctx context.Context, host host.Host, nodeRunnerID peer.ID, protocolID string) {
// 	stream, err := host.NewStream(ctx, nodeRunnerID, protocol.ID(protocolID))
// 	if err != nil {
// 		log.Fatalf("Failed to open stream to Node Runner: %v", err)
// 	}
// 	defer stream.Close()

//		log.Info("Receiving message from Node Runner")
//		buf := make([]byte, 1024)
//		n, err := stream.Read(buf)
//		if err != nil && err != io.EOF {
//			log.Errorf("Error reading from stream: %v", err)
//		}
//		log.Infof("Received message: %s", string(buf[:n]))
//	}
func handleStream(stream network.Stream) {
	log.Infof("%s: Received stream status request from %s. Node guid: %s", stream.Conn().LocalPeer(), stream.Conn().RemotePeer())
	log.Error("NEW STREAM!!!!!")
	peerID := stream.Conn().RemotePeer()
	log.Infof("relay got new stream from %s", peerID)
	log.Infof("direction, opened, limited: %v", stream.Stat())

	defer stream.Close()

	// 12D3KooWJuteouY1d5SYFcAUAYDVPjFD8MUBgqsdjZfBkAecCS2Y
	// Return a string to the mobile client

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

func main() {
	initializeLogger()
	identify.ActivationThresh = 1

	listenPort, bootstrapPeers, keyIndex, noderunnerIDStr := parseCommandLineArgs()

	if noderunnerIDStr == "" {
		log.Fatal("Node Runner Peer ID must be provided using the -noderunner flag")
	}

	// nodeRunnerID, err := peer.Decode(noderunnerIDStr)
	// if err != nil {
	// 	log.Fatalf("Invalid Node Runner Peer ID '%s': %v", noderunnerIDStr, err)
	// }

	relayOpt := getRelayIdentity(keyIndex)

	ctx := context.Background()

	host := createHost(ctx, relayOpt, listenPort)

	relayService := setupRelayService(host)

	log.Info(relayService)
	logHostInfo(host)

	kademliaDHT := createDHT(ctx, host)

	bootstrapDHT(ctx, kademliaDHT)

	connectToBootstrapPeers(ctx, host, bootstrapPeers)

	time.Sleep(5 * time.Second)

	logHostInfo(host)

	setupDHTRefresh(kademliaDHT)

	// host.SetStreamHandler(NodeRunnerProtocol, func(s network.Stream) {
	// 	handleStream(strea)
	// })
	host.SetStreamHandler(protocol.ID(NodeRunnerProtocol), handleStream)

	log.Info("Relay is ready to handle Node Runner ID requests")

	select {}
}
