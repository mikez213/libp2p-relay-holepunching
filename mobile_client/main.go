package main

import (
	"context"
	"fmt"
	"io"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"

	logging "github.com/ipfs/go-log/v2"
	libp2p "github.com/libp2p/go-libp2p"

	routing "github.com/libp2p/go-libp2p/core/routing"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"

	"github.com/libp2p/go-libp2p/p2p/host/autorelay"

	// "github.com/libp2p/go-libp2p/p2p/protocol/ping"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/libp2p/go-libp2p/core/protocol"

	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	ping "github.com/mikez213/libp2p-relay-holepunching/ping"
	cmn "github.com/mikez213/libp2p-relay-holepunching/shared"

	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("mobile_client_log")
var rend = "/customprotocol/1.0.0"

// var RelayerPrivateKeys = []string{
// 	//boots
// 	"CAESQAA7xVQKsQ5VAC5ge+XsixR7YnDkzuHa4nrY8xWXGK3fo9yN1Eaiat9Vn1iwaVQDqTjywVP303ojVLxXcQ9ze4E=",
// 	// pid: 12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n
// 	"CAESQMCYbjRpXBDUnIpDyqY+mA3n7z9gF3CaggWTknd90LauHUcz8ldNtlUchFATmMSE1r/NMnSpEBbLvzWQKq3N45s=",
// 	// pid: 12D3KooWBnext3VBZZuBwGn3YahAZjf49oqYckfx64VpzH6dyU1p
// 	"CAESQB1Y1Li0Wd4KcvMvbv5/+CTG79axzl3R8yTuzWOckMgmNAzZqxim5E/7e9mgd87FTMPQNHqiItqTFwHJeMxr0H8=",
// 	// pid: 12D3KooWDKYjXDDgSGzhEYWYtDvfP9pMtGNY1vnAwRsSp2CwCWHL

// 	//relays
// 	"CAESQHMEeM3iNIIxNThxIfnuO5FJ0oUQJy8V7TFD80lGziBE7SuPw2wckCrFRihVDaw0e6PkDCwsh/6u3UgBxB3OTFo=",
// 	//12D3KooWRnBKUEkAEpsoCoEiuhxKBJ5j2Bdop6PGxFMvd4PwoevM
// 	"CAESQP3Pu7TVp2RSVIZykj65/MDXm/eiTOfLGH3xCWQVmUoC67MkFWUEOd6QERl1Y4Xvi1Rt+d36UuaFXanT+hVUDAY=",
// 	//12D3KooWRgSQnguL2DYkXUXqCLiRQ35PEX4eEH3havy2X18AVALd
// 	"CAESQDE2IToG5mWwzWEeXt3/OVbx9XyE743DTenPFUG8M06IQXSarkNhuxNEJisnWeuDvaoaM/fNJNMqhPR81NL3Pio=",
// 	//12D3KooWEDso33ti9KsKmD2g2egNmw6BXgch7V5vFz1TziuNYybo

// 	//nodes
// 	"CAESQFffsVM3eUXLozmXkBM2FSSVhEmo/Cq5RlXOAAaniTdCu3EQ6Zf7lQDasCj6IXyTihFQWZB+nmGFn/ZAA5y5egk=",
// 	//12D3KooWNS4QQxwNURwoYoXmGjH9AQkagcGTjRUQT33P4i4FKQsi
// 	"CAESQCSHrfyzNZkxwoNmXI1wx5Lvr6o4+kGxGepFH0AfYlKthyON+1hQRjLJQaBAQLrr1cfMHFFoC40X62DQIhL246U=",
// 	//12D3KooWJuteouY1d5SYFcAUAYDVPjFD8MUBgqsdjZfBkAecCS2Y
// 	"CAESQDyiSqC9Jez8wKSQs74YJalAegamjVKHbnaN35pfe6Gk21WVgCzfvBdLVoRj8XXny/k1LtSOhPZWNz0rWKCOYpk=",
// 	//12D3KooWQaZ9Ppi8A2hcEspJhewfPqKjtXu4vx7FQPaUGnHXWpNL
// }

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
		cmn.BootstrapPeerIDs = append(cmn.BootstrapPeerIDs, pid)
	}

	// logging.SetAllLoggers(logging.LevelWarn)
	logging.SetAllLoggers(logging.LevelDebug)

	// logging.SetLogLevel("dht", "error") // get rid of  network size estimator track peers: expected bucket size number of peers
	logging.SetLogLevel("mobile_client_log", "debug")
}

func pingPeer(ctx context.Context, host host.Host, pid peer.ID, rend string, connectedPeers map[peer.ID]peer.AddrInfo) {
	log.Infof("attempting to open ping stream to %s", pid)
	log.Info(cmn.Shearing)
	// done := make(chan bool)
	// node := ping.NewNode(host, done)
	// done := ping.Ping(ctx, host, pid)
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
	log.Infof("mobile client got new stream from %s", peerID)
	log.Infof("direction, opened, limited: %+v", stream.Stat())

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

func createHost(ctx context.Context, nodeOpt libp2p.Option, relayInfo *peer.AddrInfo) (host.Host, *dht.IpfsDHT) {
	mt := autorelay.NewMetricsTracer()
	var kademliaDHT *dht.IpfsDHT

	ListenAddrs := func(cfg *config.Config) error {
		addrs := []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/::/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic", // enable QUIC
			"/ip6/::/udp/0/quic",
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
		// libp2p.EnableAutoNATService(),
		libp2p.ForceReachabilityPrivate(),
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

func searchForPeers(ctx context.Context, kademliaDHT *dht.IpfsDHT, rend string) (<-chan peer.AddrInfo, error) {
	log.Info("searching for peers")
	routingDiscovery := drouting.NewRoutingDiscovery(kademliaDHT)
	peerChan, err := routingDiscovery.FindPeers(ctx, rend)
	if err != nil {
		log.Fatalf("failed to find peers: %v", err)
	}
	return peerChan, nil
}

func connectToPeers(ctx context.Context, host host.Host, relayAddresses []peer.AddrInfo, pChan <-chan peer.AddrInfo, connectedPeers map[peer.ID]peer.AddrInfo, rend string) {
	for p := range pChan {
		if p.ID == host.ID() {
			fmt.Printf("host.ID: %v\n", host.ID())
			continue
		}
		if cmn.IsBootstrapPeer(p.ID) || cmn.IsInvalidTarget(relayAddresses, p.ID) {
			fmt.Printf("isBootstrapPeer: %v\n", cmn.IsBootstrapPeer(p.ID))
			fmt.Printf("isRelayPeer: %v\n", cmn.ContainsPeer(relayAddresses, p.ID))
			continue
		}

		val, exists := connectedPeers[p.ID]
		if exists {
			fmt.Printf("connectedPeers: %v %v\n", connectedPeers[p.ID], val)
			fmt.Printf("retry discovery for now, should skip later")
			//contune
		}

		// if connectedPeers[p.ID] != nil {
		// 	fmt.Printf("connectedPeers: %v\n", connectedPeers[p.ID])
		// 	fmt.Printf("retry discovery for now, should skip later")

		// 	// continue
		// }

		log.Errorf("!!!!!\nfound peer:\n%v", p)

		err := host.Connect(ctx, p)
		if err != nil {
			log.Warningf("failed to connect to peer directly %s : %v", p.ID, err)
			log.Infof("we have these relayAddresses:%d , %v", len(relayAddresses), relayAddresses)

			for _, relayAddrInfo := range relayAddresses {
				targetRelayedInfo, err := cmn.AssembleRelay(relayAddrInfo, p)
				if err != nil {
					log.Errorf("error in assembleRelay for peer %s: %+v", p, err)
					continue
				}

				if err := host.Connect(context.Background(), targetRelayedInfo); err != nil {
					log.Errorf("Connection failed via relay: %v", err)
					continue
				}
				// if err := host.Connect(ctx, relayAddrInfo); err != nil {
				// 	log.Warningf("failed to connect to peer %s via relay %s: %v", p.ID, relayAddrInfo.ID, err)
				// 	continue
				// }

				// add a timeout?
				log.Infof("we have connected to peer %s via relay %s", p.ID, relayAddrInfo.ID)

				log.Infof("peerswithaddrs: %v", host.Peerstore().PeersWithAddrs())

				// old streaming method

				// manualStream(ctx, host, p, relayAddrInfo)

				// var node_runner_ID peer.ID = "12D3KooWJuteouY1d5SYFcAUAYDVPjFD8MUBgqsdjZfBkAecCS2Y"
				// log.Infof("peers info of node runner: %v", host.Peerstore().SupportsProtocols(node_runner_ID, protocol.ID(rend)))

				/*
					OBVIOUSLY WE DONT HAVE A READ HERE SO WE WON"T SEE ANY RESPONSE HERE
					USE pingPeer to get messages back :P :clown:
				*/
				log.Infof("skipping TEST stream")
				log.Infof("adding %+v to connectedPeers ID %s", targetRelayedInfo, p.ID)
				connectedPeers[p.ID] = targetRelayedInfo

				// break
			}
		} else {
			_, err := host.NewStream(ctx, p.ID, protocol.ID(rend))
			if err != nil {
				log.Warningf("failed to open stream to peer directly %s: %v", p.ID, err)
				continue
			}

			log.Infof("connected to %s", p.ID)

			// stream.Close()

			connectedPeers[p.ID] = p
		}

	}
}

func manualStream(ctx context.Context, host host.Host, p peer.AddrInfo, relayAddrInfo peer.AddrInfo) {
	for i := 0; i < 5; i++ {
		log.Infof("THIS IS MANUAL TEST STREAM")
		stream, err := host.NewStream(ctx, p.ID, protocol.ID(rend))
		log.Infof("attempting to open stream to peer %s with ID %s", p.ID, protocol.ID(rend))

		// stream, err := host.NewStream(network.WithAllowLimitedConn(context.Background(), rend), targetRelayedInfo.ID, protocol.ID(rend))

		if err != nil {
			log.Warningf("failed to open stream to peer %s: %v, attempt %d/5", p.ID, err, i)
			continue
		}
		log.Infof("succesfully streaming to %s via relay %s", p.ID, relayAddrInfo.ID)

		log.Infof("protocol %s", stream.Protocol())

		state := stream.Conn().ConnState()
		log.Infof("connection id: %s, state: %+v", stream.Conn().ID(), state)

		if _, err := fmt.Fprintf(stream, "TEST\n"); err != nil {
			log.Errorf("failed to send ping to %s: %v", relayAddrInfo, err)
			return
		}
		log.Infof("TEST message sent to %s", relayAddrInfo)
		log.Infof("%s: sent stream status request from %s. Node guid: %s", stream.Conn().RemotePeer(), stream.Conn().RemotePeer())
		log.Infof("direction, opened, limited: %v", stream.Stat())
		// stream.Read()
		// stream.Write([]byte "test)
		stream.Close()
		break
	}

}

func Addresses(host host.Host) []string {

	addrs := host.Addrs()
	out := make([]string, 0, len(addrs))

	hostID := host.ID()

	for _, addr := range addrs {
		addr := fmt.Sprintf("%s/p2p/%s", addr.String(), hostID.String())
		out = append(out, addr)
	}

	return out
}

func hostGetAddrInfo(host *host.Host) *peer.AddrInfo {

	addresses := Addresses(*host)

	addr := addresses[0]

	maddr, err := multiaddr.NewMultiaddr(addr)

	info, err := peer.AddrInfoFromP2pAddr(maddr)

	if err != nil {
		log.Error(err)
	}
	return info
}

func info_dump(host *host.Host) {
	log.Info()
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

	relayAddrStr, keyIndexInt, bootstrapAddrs := cmn.ParseCmdArgs()

	nodeOpt := cmn.GetLibp2pIdentity(keyIndexInt)

	relayInfo := cmn.ParseRelayAddress(relayAddrStr)

	bootstrapPeers := cmn.ParseBootstrap(bootstrapAddrs)
	if len(bootstrapPeers) == 0 {
		log.Fatal("no valid bootstrap addrs")
	}

	// cfg := rcmgr.PartialLimitConfig{
	// 	System: rcmgr.ResourceLimits{
	// 		// Allow unlimited inbound and outbound streams
	// 		StreamsInbound:  rcmgr.Unlimited,
	// 		StreamsOutbound: rcmgr.Unlimited,
	// 	},
	// 	// Everything else is default. The exact values will come from `scaledDefaultLimits` above.
	// }

	// rm, err := rcmgr.NewResourceManager(limiter, rcmgr.WithMetricsDisabled())
	// if err != nil {
	// 	panic(err)
	// }

	host, kademliaDHT := createHost(ctx, nodeOpt, relayInfo)
	host.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			log.Infof("network We have Connected to %s", conn.RemotePeer())
		},
		DisconnectedF: func(n network.Network, conn network.Conn) {
			log.Warnf("network We have Disconnected to %s", conn.RemotePeer())
		},
	})

	// rend := "/ping"
	// rend := "/ipfs/id/1.0.0"
	identify.ActivationThresh = 1
	// rend := identify.ID
	host.SetStreamHandler(protocol.ID(rend), handleStream)
	// rend := "/ipfs/ping/1.0.0"
	// rend := ping.ID

	cmn.ConnectToBootstrapPeers(ctx, host, bootstrapPeers)
	cmn.BootstrapDHT(ctx, kademliaDHT)
	cmn.ConnectToRelay(ctx, host, relayInfo)
	relayAddresses := cmn.ConstructRelayAddresses(host, relayInfo)

	host.SetStreamHandler(protocol.ID(rend), handleStream)

	log.Infof("waiting 10 sec for stability")
	time.Sleep(5 * time.Second)

	// cmn.ReserveRelay(ctx, host, relayInfo)
	time.Sleep(5 * time.Second)

	announceSelf(ctx, kademliaDHT, rend)

	done := make(chan bool)
	pingprotocol := ping.NewPingProtocol(host, done)
	// host.SetStreamHandler(protocol.ID(rend), handleStream)

	peerChan, err := searchForPeers(ctx, kademliaDHT, rend)
	if err != nil {
		log.Fatalf("failed to find peers: %v", err)
	}

	connectedPeers := make(map[peer.ID]peer.AddrInfo)

	connectToPeers(ctx, host, relayAddresses, peerChan, connectedPeers, rend)

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
			host.Peerstore().Peers()
			log.Infof("network peers, %s, peerstore peers, %s", host.Network().Peers(), host.Peerstore().Peers())
			if len(peers) == 0 {
				log.Warn("no peers to ping")
				continue
			}

			log.Infof("current hostGetAddrInfo: %+v", hostGetAddrInfo(&host))

			for _, peerID := range peers {
				if peerID == host.ID() || cmn.IsInvalidTarget(relayAddresses, peerID) {
					continue
				}
				log.Infof("Pinging peer: %s", peerID)
				// go func(pid peer.ID) {
				log.Infof("mass sending protocols to %s", peerID)
				// if host.Network().Connectedness(peerID) != network.Connected {
				log.Infof("Connected level of %s is %+v", peerID, host.Network().Connectedness(peerID))
				// continue
				// }
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
				// }(peerID)
				// go pingPeer(ctx, host, peerID, rend, connectedPeers, pingprotocol)
			}
		}
	}()

	time.Sleep(3 * time.Second)

	select {}
}
