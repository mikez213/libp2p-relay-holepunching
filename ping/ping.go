package customprotocol

import (
	"context"
	"fmt"
	"io"
	"time"

	// "log"
	"sync"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	p2p "github.com/mikez213/libp2p-relay-holepunching/ping/pb"
	proto "google.golang.org/protobuf/proto"
)

var log = logging.Logger("ping-log")

var isStreaming = false

/*
generate in /ping:
protoc --go_out=. --go_opt=paths=source_relative pb/p2p.proto
*/

// node client version
const clientVersion = "go-p2p-node/0.0.1"

// pattern: /protocol-name/request-or-response-message/version
const (
	pingRequest         = "/ping/pingreq/0.0.1"
	pingResponse        = "/ping/pingresp/0.0.1"
	startStreamRequest  = "/stream/startstreamreq/0.0.1"
	startStreamResponse = "/stream/startstreamresp/0.0.1"
	stopStreamRequest   = "/stream/stopstreamreq/0.0.1"
	stopStreamResponse  = "/stream/stopstreamresp/0.0.1"
	statusRequest       = "/status/statusreq/0.0.1"
	statusResponse      = "/status/statusresp/0.0.1"
	infoRequest         = "/info/identreq/0.0.1" //not identify that would collide
	infoResponse        = "/info/identresp/0.0.1"
)

type ProtocolHandler interface {
	Handle(s network.Stream, from peer.ID, data []byte) error
}

// PingProtocol type
type PingProtocol struct {
	host             host.Host
	mu               sync.Mutex
	requestHandlers  map[protocol.ID]ProtocolHandler
	responseHandlers map[protocol.ID]ProtocolHandler
	// requests map[string]*p2p.PingRequest // used to access request data from response handlers. Protected by mu
	done chan bool // only for demo purposes to stop main from terminating
}

func NewPingProtocol(host host.Host, done chan bool) *PingProtocol {
	p := &PingProtocol{host: host,
		// requests: make(map[string]*p2p.PingRequest),
		done:             done,
		requestHandlers:  make(map[protocol.ID]ProtocolHandler),
		responseHandlers: make(map[protocol.ID]ProtocolHandler),
	}
	logging.SetLogLevel("ping-log", "debug")

	p.registerRequestHandler(pingRequest, &PingRequestHandler{protocol: p})
	p.registerRequestHandler(startStreamRequest, &StartStreamRequestHandler{protocol: p})
	p.registerRequestHandler(stopStreamRequest, &StopStreamRequestHandler{protocol: p})
	p.registerRequestHandler(statusRequest, &StatusRequestHandler{protocol: p})
	p.registerRequestHandler(infoRequest, &InfoRequestHandler{protocol: p})

	p.registerResponseHandler(pingResponse, &PingResponseHandler{protocol: p})
	p.registerResponseHandler(startStreamResponse, &StartStreamResponseHandler{protocol: p})
	p.registerResponseHandler(stopStreamResponse, &StopStreamResponseHandler{protocol: p})
	p.registerResponseHandler(statusResponse, &StatusResponseHandler{protocol: p})
	p.registerResponseHandler(infoResponse, &InfoResponseHandler{protocol: p})

	// requests := []string{
	// 	pingRequest,
	// 	startStreamRequest,
	// 	stopStreamRequest,
	// 	statusRequest,
	// 	infoRequest,
	// }
	// for _, pid := range requests {
	// 	p.host.SetStreamHandler(protocol.ID(pid), p.onProtocolRequest)
	// }

	// // Register all response handlers to onProtocolResponse
	// responses := []string{
	// 	pingResponse,
	// 	startStreamResponse,
	// 	stopStreamResponse,
	// 	// statusResponse,
	// 	// infoResponse,
	// }
	// for _, pid := range responses {
	// 	p.host.SetStreamHandler(protocol.ID(pid), p.onProtocolResponse)
	// }
	log.Debugf("protocols: %+v, %+v", p.requestHandlers, p.responseHandlers)
	return p
}

func (p *PingProtocol) registerRequestHandler(protocolID protocol.ID, handler ProtocolHandler) {
	p.requestHandlers[protocolID] = handler
	p.host.SetStreamHandler(protocol.ID(protocolID), p.onProtocolRequest)
}

func (p *PingProtocol) registerResponseHandler(protocolID protocol.ID, handler ProtocolHandler) {
	p.responseHandlers[protocolID] = handler
	p.host.SetStreamHandler(protocol.ID(protocolID), p.onProtocolResponse)
}

func (p *PingProtocol) onProtocolRequest(s network.Stream) {
	defer s.Close()

	cur_protocol := s.Protocol()
	log.Debugf("onProtocolRequest called with protocol: %s", cur_protocol)

	handler, exists := p.requestHandlers[cur_protocol]
	if !exists {
		log.Errorf("no handler for protocol: %s", cur_protocol)
		log.Errorf("avalibe protocols: %+v, handler here should be nil :'%+v' ", p.requestHandlers, handler)
		s.Reset()
		return
	}

	// Read the incoming message
	buf, err := io.ReadAll(s)
	if err != nil {
		log.Error(err, "Failed to read incoming request")
		s.Reset()
		return
	}

	err = handler.Handle(s, s.Conn().RemotePeer(), buf)
	if err != nil {
		log.Errorf("Error handling request for protocol %s: %v", cur_protocol, err)
		s.Reset()
		return
	}
}

func (p *PingProtocol) onProtocolResponse(s network.Stream) {
	defer s.Close()

	cur_protocol := s.Protocol()
	log.Debugf("onProtocolResponse called with protocol: %s", cur_protocol)

	handler, exists := p.responseHandlers[cur_protocol]
	if !exists {
		log.Errorf("no handler for protocol: %s", cur_protocol)
		log.Errorf("avalibe protocols: %+v, handler here should be nil :'%+v' ", p.requestHandlers, handler)
		s.Reset()
		return
	}

	// Read the incoming message
	buf, err := io.ReadAll(s)
	if err != nil {
		log.Error(err, "Failed to read incoming request")
		s.Reset()
		return
	}

	err = handler.Handle(s, s.Conn().RemotePeer(), buf)
	if err != nil {
		log.Errorf("Error handling request for protocol %s: %v", cur_protocol, err)
		s.Reset()
		return
	}
}

// REAL FUNCTIONS

// ping sends minimal ping string to peer to check connection
func (p *PingProtocol) Ping(target peer.ID) bool {
	log.Infof("%s: Sending ping to: %s....", p.host.ID(), target)

	// create message data
	req := &p2p.PingRequest{
		MessageData: fmt.Sprintf("ping prot from %s at %s", p.host.ID(), time.Now().Format(time.RFC3339)),
		Message:     fmt.Sprintf("hello prot from %s!", p.host.ID()),
	}

	// Send the ping request using the Ping Protocol
	ok := p.sendProtoMessage(target, pingRequest, req)
	if !ok {
		return false
	}

	log.Infof("%s: Ping to: %s was sent. Message ID: %s, Message: %s", p.host.ID(), target, req.MessageData, req.Message)
	return true
}

// StartStream sends a requests a stream with some configs to a target peer
func (p *PingProtocol) StartStream(target peer.ID, projectID, devID, apiKey, issueNeed string, configOptions map[string]string) bool {
	log.Infof("%s: Sending StartStreamRequest to: %s....", p.host.ID(), target)

	// Create StartStreamRequest
	req := &p2p.StartStreamRequest{
		Id: &p2p.Id{
			ProjectId: projectID,
			DevId:     devID,
			ApiKey:    apiKey,
		},
		RequestIssueNeed: issueNeed,
		ConfigOptions:    configOptions,
	}

	// Send StartStreamRequest
	ok := p.sendProtoMessage(target, startStreamRequest, req)
	if !ok {
		return false
	}

	log.Infof("StartStreamRequest sent to: %s. ProjectID: %s, DevID: %s, APIKey: %s, IssueNeed: %s", target, projectID, devID, apiKey, issueNeed)
	return true
}

// StopStream is a request to stop the stream
func (p *PingProtocol) StopStream(target peer.ID, projectID, devID, apiKey string) bool {
	log.Infof("%s: Sending StopStreamRequest to: %s....", p.host.ID(), target)

	req := &p2p.StopStreamRequest{
		Id: &p2p.Id{
			ProjectId: projectID,
			DevId:     devID,
			ApiKey:    apiKey,
		},
	}

	ok := p.sendProtoMessage(target, stopStreamRequest, req)
	if !ok {
		return false
	}

	log.Infof("StopStreamRequest sent to: %s. ProjectID: %s, DevID: %s, APIKey: %s", target, projectID, devID, apiKey)
	return true
}

// Status asks if the target is already stream a project, and has some basic status info
func (p *PingProtocol) Status(target peer.ID, projectID, devID, apiKey string) bool {
	log.Infof("%s: Sending StatusRequest to: %s....", p.host.ID(), target)

	req := &p2p.StatusRequest{
		Id: &p2p.Id{
			ProjectId: projectID,
			DevId:     devID,
			ApiKey:    apiKey,
		},
	}

	ok := p.sendProtoMessage(target, statusRequest, req)
	if !ok {
		return false
	}

	log.Infof("statusRequest sent to: %s. ProjectID: %s, DevID: %s, APIKey: %s", target, projectID, devID, apiKey)
	return true
}

// Info asks for addresses, connectivity, and hardware of target
func (p *PingProtocol) Info(target peer.ID, hostID string) bool {
	log.Infof("%s: Sending InfoRequest to: %s....", p.host.ID(), target)

	req := &p2p.InfoRequest{
		HostId: hostID,
	}

	ok := p.sendProtoMessage(target, infoRequest, req)
	if !ok {
		return false
	}

	log.Infof("InfoRequest sent to: %s. hostid: %s", target, req.HostId)
	return true
}

// helper method - writes a protobuf go data object to a network stream
// data: reference of protobuf go data object to send (not the object itself)
// s: network stream to write the data to
func (p *PingProtocol) sendProtoMessage(id peer.ID, pid protocol.ID, data proto.Message) bool {
	log.Info(p.host.Peerstore().PeersWithAddrs())
	s, err := p.host.NewStream(context.Background(), id, pid)
	// network.WithAllowLimitedConn(context.Background(), string(pid))
	if err != nil {
		log.Error(err)
		return false
	}
	defer s.Close()

	bytes, err := proto.Marshal(data)
	if err != nil {
		log.Error(err)
		return false
	}

	n, err := s.Write(bytes)
	if err != nil {
		log.Errorf("%d, '%+v'", n, err)
		s.Reset()

		retry := true
		if retry {
			log.Warnf("retry protoMessage is true, trying to stream again!")
			peers := p.host.Network().Peers()
			log.Debugf("peers %s=V", peers)
			time.Sleep(2 * time.Second)
			stream, err := p.host.NewStream(network.WithAllowLimitedConn(context.Background(), string(pid)), id, pid)
			if err != nil {
				log.Error(err)
			}
			defer stream.Close()
			n, err := stream.Write(bytes)
			if err != nil {
				log.Errorf("%d, '%+v'", n, err)
				stream.Reset()
				return false
			}
		} else {
			return false
		}
	}
	return true
}
