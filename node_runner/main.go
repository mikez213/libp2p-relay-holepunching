package main

import (
	"bufio"
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

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"

	ping "github.com/mikez213/libp2p-relay-holepunching/ping"
	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("node_runner_log")

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
	// logging.SetAllLoggers(logging.LevelDebug)

	logging.SetAllLoggers(logging.LevelWarn)
	// logging.SetLogLevel("dht", "error") // get rid of  network size estimator track peers: expected bucket size number of peers
	logging.SetLogLevel("node_runner_log", "debug")
}

func pingPeer(ctx context.Context, host host.Host, pid peer.ID, rend string, connectedPeers map[peer.ID]peer.AddrInfo, pingprotocol ping.PingProtocol) {
	log.Infof("attempting to open ping stream to %s", pid)

	pingprotocol.Ping(pid)

	// // Send Ping
	// node.PingProtocol.Ping(pid)
	// Wait for Ping Response
	// select {
	// case <-done:
	// 	log.Infof("Ping exchange completed")
	// 	log.Infof("%v", done)
	// case <-time.After(5 * time.Second):
	// 	log.Errorf("Ping exchange timed out")
	// 	log.Errorf("%v", done)
	// }

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
		// connectedPeers[pid] = true
	} else {
		log.Warnf("Unexpected response from %s: str'%s' byte'%08b'", pid, response, response)
	}
}

func handleStream(stream network.Stream) {
	log.Infof("%s: Received stream status request from %s. Node guid: %s", stream.Conn().LocalPeer(), stream.Conn().RemotePeer())

	c := stream.Conn()
	log.Debugf("%s received message from %s %s", stream.Protocol(), c.RemotePeer(), c.RemoteMultiaddr())

	log.Error("NEW STREAM!!!!!")
	peerID := stream.Conn().RemotePeer()
	log.Infof("node runner got new stream from %s", peerID)
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

		if received == "PING\n" || received == "TEST\n" {
			log.Infof("received %s from %s, responding with PONG", received, peerID)
			if _, err = fmt.Fprintf(stream, "PONG\n"); err != nil {
				log.Errorf("error writing PONG to stream: %v", err)
				return
			}
			log.Infof("sent PONG to %s", peerID)

			rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
			out, err := rw.WriteString("noder")
			if err != nil {
				log.Errorf("error writing noder to stream: bytes(%d), %v", out, err)
			}
			log.Infof("sent noder via bufio")

		} else {
			log.Warnf("unexpected message from %s: %s", peerID, received)
		}

		break
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
	// prevents this error:
	// DEBUG   rcmgr   resource-manager/scope.go:480
	// blocked stream from constraining edge
	// {"scope": "stream-907", "edge": "peer:12D3KooWJuteouY1d5SYFcAUAYDVPjFD8MUBgqsdjZfBkAecCS2Y",
	//"direction": "Inbound", "current": 378, "attempted": 1, "limit": 378, "stat":
	//{"NumStreamsInbound":378,"NumStreamsOutbound":0,"NumConnsInbound":0,"NumConnsOutbound":1,"NumFD":0,"Memory":99352576},
	// "error": "peer:12D3KooWJuteouY1d5SYFcAUAYDVPjFD8MUBgqsdjZfBkAecCS2Y:
	//cannot reserve inbound stream: resource limit exceeded"}
	rcmgr, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(rcmgr.InfiniteLimits))
	if err != nil {
		log.Fatalf("could not create new resource manager: %w", err)
	}

	host, err := libp2p.New(
		nodeOpt,
		ListenAddrs,
		libp2p.EnableRelay(),
		libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{*relayInfo}, autorelay.WithMetricsTracer(mt)),
		libp2p.NATPortMap(),
		libp2p.EnableAutoNATv2(),
		// libp2p.ForceReachabilityPrivate(),
		libp2p.EnableNATService(),
		libp2p.EnableHolePunching(),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			kademliaDHT, _ = dht.New(ctx, h, dht.Mode(dht.ModeClient))
			return kademliaDHT, nil
		}),
		libp2p.ResourceManager(rcmgr),
	)
	if err != nil {
		log.Fatalf("failed to create libp2p host: %v", err)
	}

	log.Infof("host created, we are %s", host.ID())

	return host, kademliaDHT
}

// func setupStreamHandler(host host.Host, rend string) {
// 	host.SetStreamHandler(protocol.ID(rend), handleStream)
// }

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

func containsPeer(relayAddresses []peer.AddrInfo, pid peer.ID) bool {
	for _, relayAddrInfo := range relayAddresses {
		if relayAddrInfo.ID == pid {
			return true
		}
	}
	return false
}

func reserveRelay(ctx context.Context, host host.Host, relayInfo *peer.AddrInfo) (*client.Reservation, error) {
	var reservation *client.Reservation
	reservation, err := client.Reserve(ctx, host, *relayInfo)
	if err != nil {
		log.Errorf("failed to receive a relay reservation from relay %v", err)
		return reservation, err
	}
	log.Infof("relay reservation successful")
	log.Debugf("All reservation info: %+v", reservation)
	log.Debugf("Voucher relay info: %+v", reservation.Voucher.Relay.Loggable())
	log.Debugf("Voucher reservation info: %+v", reservation.Voucher)

	return reservation, nil
}

func announceSelf(ctx context.Context, kademliaDHT *dht.IpfsDHT, rend string) {
	log.Info("announcing ourselves")
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	dutil.Advertise(ctx, routingDiscovery, rend)
	log.Info("successfully announced")
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

	rend := "/customprotocol/1.0.0"
	// rend := "/ipfs/id/1.0.0"
	identify.ActivationThresh = 1
	// setupStreamHandler(host, rend)
	host.SetStreamHandler(protocol.ID(rend), handleStream)

	connectToBootstrapPeers(ctx, host, bootstrapPeers)
	bootstrapDHT(ctx, kademliaDHT)
	connectToRelay(ctx, host, relayInfo)
	relayAddresses := constructRelayAddresses(host, relayInfo)

	log.Infof("waiting 10 sec for stability")
	time.Sleep(5 * time.Second)

	reserveRelay(ctx, host, relayInfo)
	time.Sleep(5 * time.Second)

	done := make(chan bool)

	pingprotocol := ping.NewPingProtocol(host, done)

	announceSelf(ctx, kademliaDHT, rend)

	projectID := "project_test_1234"
	devID := "dev_1234"
	apiKey := "api_1234"
	issueNeed := "issue_1234"
	hostID := "host_1234"
	configOptions := map[string]string{"val1": "key1", "val2": "key2"}

	ticker := time.NewTicker(7 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			peers := host.Network().Peers()
			if len(peers) == 0 {
				log.Warn("no peers to ping")
				continue
			}

			for _, peerID := range peers {
				if peerID == host.ID() || isBootstrapPeer(peerID) || containsPeer(relayAddresses, peerID) {
					continue
				}
				// log.Info("WOULD PING HERE BUT CANCELED @@@@@")
				// go pingPeer(ctx, host, peerID, rend, connectedPeers, pingprotocol)
				log.Infof("protocol actions for: %s", peerID)

				log.Info("mass sending protocols to %s", peerID)
				pingprotocol.Ping(peerID)
				pingprotocol.Status(peerID, projectID, devID, apiKey)
				pingprotocol.Info(peerID, hostID)
				pingprotocol.StartStream(peerID, projectID, devID, apiKey, issueNeed, configOptions)
				time.Sleep(1 * time.Second)
				pingprotocol.Status(peerID, projectID, devID, apiKey)
				pingprotocol.StopStream(peerID, projectID, devID, apiKey)

				select {
				case <-done:
					log.Infof(" exchange completed")

				case <-time.After(5 * time.Second):
					log.Errorf(" exchange timed out")
				}
			}
		}
	}()

	select {}
}
