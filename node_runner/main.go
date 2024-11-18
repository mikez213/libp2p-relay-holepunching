package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/config"
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
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"

	ping "github.com/mikez213/libp2p-relay-holepunching/ping"
	cmn "github.com/mikez213/libp2p-relay-holepunching/shared"

	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("node_runner_log")

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

func announceSelf(ctx context.Context, kademliaDHT *dht.IpfsDHT, rend string) {
	log.Info("announcing ourselves")
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	dutil.Advertise(ctx, routingDiscovery, rend)
	log.Info("successfully announced")
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	relayAddrStr, keyIndexInt, bootstrapAddrs := cmn.ParseCmdArgs()

	nodeOpt := cmn.GetLibp2pIdentity(keyIndexInt)

	relayInfo := cmn.ParseRelayAddress(relayAddrStr)

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

	cmn.ConnectToBootstrapPeers(ctx, host, bootstrapPeers)
	cmn.BootstrapDHT(ctx, kademliaDHT)
	cmn.ConnectToRelay(ctx, host, relayInfo)
	relayAddresses := cmn.ConstructRelayAddresses(host, relayInfo)

	log.Infof("waiting 10 sec for stability")
	time.Sleep(5 * time.Second)

	cmn.ReserveRelay(ctx, host, relayInfo)
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
				if peerID == host.ID() || cmn.IsInvalidTarget(relayAddresses, peerID) {
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
