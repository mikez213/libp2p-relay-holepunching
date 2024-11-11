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

	proto "github.com/gogo/protobuf/proto"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	p2p "github.com/mikez213/libp2p-relay-holepunching/ping/pb"
)

var log = logging.Logger("ping-log")

/*

generate in /ping:
protoc --go_out=. --go_opt=paths=source_relative pb/p2p.proto

*/

// node client version
const clientVersion = "go-p2p-node/0.0.1"

// pattern: /protocol-name/request-or-response-message/version
const pingRequest = "/ping/pingreq/0.0.1"
const pingResponse = "/ping/pingresp/0.0.1"

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
	return p
}

// remote peer requests handler
func (p *PingProtocol) onPingRequest(s network.Stream) {
	log.Debug("got ping request 11111")
	// get request data
	data := &p2p.PingRequest{}
	log.Debug("%+v", data)
	buf, err := io.ReadAll(s)
	log.Debug("%+v", buf)

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
