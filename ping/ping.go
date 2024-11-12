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

// PingProtocol type
type PingProtocol struct {
	host host.Host
	mu   sync.Mutex
	// requests map[string]*p2p.PingRequest // used to access request data from response handlers. Protected by mu
	done chan bool // only for demo purposes to stop main from terminating
}

func NewPingProtocol(host host.Host, done chan bool) *PingProtocol {
	p := &PingProtocol{host: host,
		// requests: make(map[string]*p2p.PingRequest),
		done: done}
	p.host.SetStreamHandler(pingRequest, p.onPingRequest)
	p.host.SetStreamHandler(pingResponse, p.onPingResponse)
	p.host.SetStreamHandler(startStreamRequest, p.onStartStreamRequest)
	p.host.SetStreamHandler(startStreamResponse, p.onStartStreamResponse)
	p.host.SetStreamHandler(stopStreamRequest, p.onStopStreamRequest)
	p.host.SetStreamHandler(stopStreamResponse, p.onStopStreamResponse)
	// p.host.SetStreamHandler(statusRequest, p.onStatusRequest)
	// p.host.SetStreamHandler(statusResponse, p.onStatusResponse)
	// p.host.SetStreamHandler(infoRequest, p.onInfoRequest)
	// p.host.SetStreamHandler(infoResponse, p.onInfoResponse)

	// requests := []string{
	// 	pingRequest,
	// 	startStreamRequest,
	// 	stopStreamRequest,
	// 	// statusRequest,
	// 	// infoRequest,
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

	return p
}

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

// Ping sends a ping request to a peer
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

// StartStream sends a StartStreamRequest to a target peer
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

// StartStream sends a StartStreamRequest to a target peer
func (p *PingProtocol) StopStream(target peer.ID, projectID, devID, apiKey string) bool {
	log.Infof("%s: Sending StopStreamRequest to: %s....", p.host.ID(), target)

	// Create StartStreamRequest
	req := &p2p.StopStreamRequest{
		Id: &p2p.Id{
			ProjectId: projectID,
			DevId:     devID,
			ApiKey:    apiKey,
		},
	}

	// Send StartStreamRequest
	ok := p.sendProtoMessage(target, stopStreamRequest, req)
	if !ok {
		return false
	}

	log.Infof("StopStreamRequest sent to: %s. ProjectID: %s, DevID: %s, APIKey: %s", target, projectID, devID, apiKey)
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
