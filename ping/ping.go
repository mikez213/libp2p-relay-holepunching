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
	s, err := p.host.NewStream(network.WithAllowLimitedConn(context.Background(), string(pid)), id, pid)
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
		log.Fatalf("%d, %+v", n, err)
		s.Reset()
		return false
	}
	return true
}

//// UNUSED FUNCTIONS

// remote peer requests handler
func (p *PingProtocol) onPingRequest(s network.Stream) {
	log.Debug("got ping request 11111")
	// get request data
	data := &p2p.PingRequest{}
	log.Debugf("%+v", data)
	buf, err := io.ReadAll(s)
	log.Debugf("%+v", buf)

	if err != nil {
		log.Error(err)
		s.Reset()
		return
	}
	s.Close()

	// unmarshal it
	err = proto.Unmarshal(buf, data)
	if err != nil {
		log.Errorf("%+v", err)
		return
	}

	log.Infof("%s: Received ping request from %s. Message: %s", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.Message)

	// generate response message
	log.Infof("%s: Sending ping response to %s. Message id: %s...", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.MessageData)

	resp := &p2p.PingResponse{
		MessageData: fmt.Sprintf("Response to %s", data.MessageData),
		Message:     fmt.Sprintf("Ping response from %s", p.host.ID()),
	}

	// send the response
	ok := p.sendProtoMessage(s.Conn().RemotePeer(), pingResponse, resp)

	if ok {
		log.Infof("%s: Ping response to %s sent.", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	} else {
		log.Errorf("%s: Error in Ping response to %s", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	}

	p.done <- true
}

// remote ping response handler
func (p *PingProtocol) onPingResponse(s network.Stream) {
	// data := &p2p.PingResponse{}
	buf, err := io.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Error(err)
		return
	}
	s.Close()

	var resp p2p.PingResponse
	if err := proto.Unmarshal(buf, &resp); err != nil {
		log.Error(err, "Failed to unmarshal PingResponse")
		s.Reset()
		return
	}

	log.Infof("Received PingResponse from %s: %s", s.Conn().RemotePeer(), resp.MessageData)
	p.done <- true
}

// handles incoming StartStreamRequest messages
func (p *PingProtocol) onStartStreamRequest(s network.Stream) {
	defer s.Close()
	log.Debug("Received StartStreamRequest")

	buf, err := io.ReadAll(s)
	if err != nil {
		log.Error(err, "Failed to read StartStreamRequest")
		s.Reset()
		return
	}

	var req p2p.StartStreamRequest
	if err := proto.Unmarshal(buf, &req); err != nil {
		log.Error(err, "Failed to unmarshal StartStreamRequest")
		s.Reset()
		return
	}

	log.Infof("Received StartStreamRequest from %s: ProjectID=%s, DevID=%s, APIKey=%s",
		s.Conn().RemotePeer(), req.Id.ProjectId, req.Id.DevId, req.Id.ApiKey)

	// TODO ADD ACTUAL XR STREAMING LOGIC HERE! (use channel to indicate to outsider program)
	// currenlty assume it just works

	isStreaming := true
	statusMessage := "SUCCESS" //replace str with proto value

	// Generate StartStreamResponse
	resp := &p2p.StartStreamResponse{
		Id:            &p2p.Id{ProjectId: req.Id.ProjectId, DevId: req.Id.DevId, ApiKey: req.Id.ApiKey},
		IsStreaming:   isStreaming,
		StatusMessage: statusMessage,
	}

	// send the response
	ok := p.sendProtoMessage(s.Conn().RemotePeer(), startStreamResponse, resp)

	if ok {
		log.Infof("%s: startStreamResponse to %s sent.", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	} else {
		log.Errorf("%s: $$$$$ Error in startStreamResponse to %s", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	}

	log.Infof("Sent StartStreamResponse to %s: IsStreaming=%v, StatusMessage=%s",
		s.Conn().RemotePeer(), isStreaming, statusMessage)
	p.done <- true
}

// confirming the stream has been Started
func (p *PingProtocol) onStartStreamResponse(s network.Stream) {
	// data := &p2p.PingResponse{}
	buf, err := io.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Error(err)
		return
	}
	s.Close()

	var resp p2p.StartStreamResponse
	if err := proto.Unmarshal(buf, &resp); err != nil {
		log.Error(err, "Failed to unmarshal StartStreamResponse")
		s.Reset()
		return
	}

	//TODO: add confirmation that we have gotten the actual xr stream here

	log.Infof("Received StartStreamResponse from %s: IsStreaming=%v, StatusMessage=%s", s.Conn().RemotePeer(), resp.IsStreaming, resp.StatusMessage)
	p.done <- true
}

// handles incoming StopStreamResponse messages
func (p *PingProtocol) onStopStreamRequest(s network.Stream) {
	defer s.Close()
	log.Debug("Received onStopStreamRequest")

	buf, err := io.ReadAll(s)
	if err != nil {
		log.Error(err, "Failed to read onStopStreamRequest")
		s.Reset()
		return
	}

	var req p2p.StopStreamRequest
	if err := proto.Unmarshal(buf, &req); err != nil {
		log.Error(err, "Failed to unmarshal onStopStreamRequest")
		s.Reset()
		return
	}

	log.Infof("Received onStopStreamRequest from %s: ProjectID=%s, DevID=%s, APIKey=%s",
		s.Conn().RemotePeer(), req.Id.ProjectId, req.Id.DevId, req.Id.ApiKey)

	// TODO ADD ACTUAL XR STREAMING LOGIC HERE! (use channel to indicate to outsider program)
	// currenlty assume it just works

	// Generate StartStreamResponse
	resp := &p2p.StopStreamResponse{
		Id: &p2p.Id{ProjectId: req.Id.ProjectId, DevId: req.Id.DevId, ApiKey: req.Id.ApiKey},
	}

	// send the response
	ok := p.sendProtoMessage(s.Conn().RemotePeer(), startStreamResponse, resp)

	if ok {
		log.Infof("%s: StopStreamResponse to %s sent.", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	} else {
		log.Errorf("%s: $$$$$ Error in StopStreamResponse to %s", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	}

	log.Infof("Sent StopStreamResponse to %s")
	p.done <- true
}

// confirming the stream has been Started
func (p *PingProtocol) onStopStreamResponse(s network.Stream) {
	// data := &p2p.PingResponse{}
	buf, err := io.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Error(err)
		return
	}
	s.Close()

	var resp p2p.StopStreamResponse
	if err := proto.Unmarshal(buf, &resp); err != nil {
		log.Error(err, "Failed to unmarshal StopStreamResponse")
		s.Reset()
		return
	}

	//TODO: add confirmation that we have gotten the actual xr stream here

	log.Infof("Received StopStreamResponse from %s", s.Conn().RemotePeer())
	p.done <- true
}

// var message proto.Message

// switch string(cur_protocol) {
// case pingRequest:
// 	message = &p2p.PingRequest{}
// case startStreamRequest:
// 	message = &p2p.StartStreamRequest{}
// case stopStreamRequest:
// 	message = &p2p.StopStreamRequest{}
// case statusRequest:
// 	message = &p2p.StatusRequest{}
// case infoRequest:
// 	message = &p2p.InfoRequest{}
// default:
// 	message = nil
// 	log.Errorf("Unknown request protocol ID: %s", cur_protocol)
// 	s.Reset()
// 	return
// }

// if err := proto.Unmarshal(buf, message); err != nil {
// 	log.Errorf("Failed to unmarshal message for protocol %s: %v", cur_protocol, err)
// 	s.Reset()
// 	return
// }

// log.Debugf("got message %+v", message)
// //proccessing

// var resp proto.Message
// switch msg := message.(type) {
// case *p2p.PingRequest:
// 	log.Info("type is Ping")

// 	resp = &p2p.PingResponse{
// 		MessageData: fmt.Sprintf("Response to %s", msg.MessageData),
// 		Message:     fmt.Sprintf("Ping response from %s", p.host.ID()),
// 	}

// case *p2p.StartStreamRequest:
// 	log.Info("type is Start")

// 	// logic for actuall XR streaming goes here
// 	// processStream(config data)

// 	//mock

// 	isStreaming, statusMessage := process(msg, resp, s)

// 	resp = &p2p.StartStreamResponse{
// 		Id:            &p2p.Id{ProjectId: msg.Id.ProjectId, DevId: msg.Id.DevId, ApiKey: msg.Id.ApiKey},
// 		IsStreaming:   isStreaming,
// 		StatusMessage: statusMessage,
// 	}
// case *p2p.StopStreamRequest:
// 	log.Info("type is Stop")

// 	//logic for stoppingactuall XR streaming goes here

// 	resp = &p2p.StopStreamResponse{
// 		Id: &p2p.Id{ProjectId: msg.Id.ProjectId, DevId: msg.Id.DevId, ApiKey: msg.Id.ApiKey},
// 	}

// case *p2p.StatusRequest:
// 	log.Info("type is Stat")

// case *p2p.InfoRequest:
// 	log.Info("type is Info")

// default:
// 	log.Errorf("Unhandled message type for protocol %s", cur_protocol)
// }

// // send the response
// ok := p.sendProtoMessage(s.Conn().RemotePeer(), pingResponse, resp)

// if ok {
// 	log.Infof("%s: %T response to %s sent.", s.Conn().LocalPeer().String(), message, s.Conn().RemotePeer().String())
// } else {
// 	log.Errorf("%s: Error in Ping response to %s", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
// }

// // Read the incoming message
// buf, err := io.ReadAll(s)
// if err != nil {
// 	log.Error(err, "Failed to read incoming request")
// 	s.Reset()
// 	return
// }

// switch string(cur_protocol) {
// case pingRequest:
// 	var req p2p.PingRequest
// case startStreamRequest:
// 	var req p2p.StartStreamRequest
// case stopStreamRequest:
// 	var req p2p.StopStreamRequest
// case statusRequest:
// 	var req p2p.StatusRequest
// case infoRequest:
// 	var req p2p.InfoRequest
// default:
// 	log.Errorf("Unknown request protocol : %s", cur_protocol)
// 	s.Reset()
// 	return
// }

// if err := proto.Unmarshal(buf, &req); err != nil {
// 	log.Error(err, "Failed to unmarshal PingRequest")
// 	s.Reset()
// 	return
// }

// log.Infof("Received %s from %s: %s", req, s.Conn().RemotePeer(), req.MessageData)
