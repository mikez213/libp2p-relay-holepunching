package customprotocol

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"

	proto "github.com/gogo/protobuf/proto"
	uuid "github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	p2p "github.com/mikez213/libp2p-relay-holepunching/ping/pb" // Update the import path as per your project structure

	ggio "github.com/gogo/protobuf/io"
)

/*

generate in /ping:
protoc --go_out=. --go_opt=paths=source_relative pb/p2p.proto

*/

// node client version
const clientVersion = "go-p2p-node/0.0.1"

// Node type - a p2p host implementing one or more p2p protocols
// type Node struct {
// 	host.Host     // lib-p2p host
// 	*PingProtocol // ping protocol impl
// 	// add other protocols here...
// }

// // Create a new node with its implemented protocols
// func NewNode(host host.Host, done chan bool) *Node {
// 	node := &Node{Host: host}
// 	node.PingProtocol = NewPingProtocol(node, done)
// 	return node
// }

// Authenticate incoming p2p message
// message: a protobuf go data object
// data: common p2p message data
func (p *PingProtocol) authenticateMessage(message proto.Message, data *p2p.MessageData) bool {
	// store a temp ref to signature and remove it from message data
	// sign is a string to allow easy reset to zero-value (empty string)
	sign := data.Sign
	data.Sign = nil

	// marshall data without the signature to protobufs3 binary format
	bin, err := proto.Marshal(message)
	if err != nil {
		log.Println(err, "failed to marshal pb message")
		return false
	}

	// restore sig in message data (for possible future use)
	data.Sign = sign

	// restore peer id binary format from base58 encoded node id data
	peerId, err := peer.Decode(data.NodeId)
	if err != nil {
		log.Println(err, "Failed to decode node id from base58")
		return false
	}

	// verify the data was authored by the signing peer identified by the public key
	// and signature included in the message
	return p.verifyData(bin, []byte(sign), peerId, data.NodePubKey)
}

// sign an outgoing p2p message payload
func (p *PingProtocol) signProtoMessage(message proto.Message) ([]byte, error) {
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	return p.signData(data)
}

// sign binary data using the local node's private key
func (p *PingProtocol) signData(data []byte) ([]byte, error) {
	key := p.host.Peerstore().PrivKey(p.host.ID())
	res, err := key.Sign(data)
	return res, err
}

// Verify incoming p2p message data integrity
// data: data to verify
// signature: author signature provided in the message payload
// peerId: author peer id from the message payload
// pubKeyData: author public key from the message payload
func (p *PingProtocol) verifyData(data []byte, signature []byte, peerId peer.ID, pubKeyData []byte) bool {
	key, err := crypto.UnmarshalPublicKey(pubKeyData)
	if err != nil {
		log.Println(err, "Failed to extract key from message key data")
		return false
	}

	// extract node id from the provided public key
	idFromKey, err := peer.IDFromPublicKey(key)

	if err != nil {
		log.Println(err, "Failed to extract peer id from public key")
		return false
	}

	// verify that message author node id matches the provided node public key
	if idFromKey != peerId {
		log.Println(err, "Node id and provided public key mismatch")
		return false
	}

	res, err := key.Verify(data, signature)
	if err != nil {
		log.Println(err, "Error authenticating data")
		return false
	}

	return res
}

// helper method - generate message data shared between all node's p2p protocols
// messageId: unique for requests, copied from request for responses
func (p *PingProtocol) NewMessageData(messageId string, gossip bool) *p2p.MessageData {
	// Add protobuf bin data for message author public key
	// this is useful for authenticating  messages forwarded by a node authored by another node
	nodePubKey, err := crypto.MarshalPublicKey(p.host.Peerstore().PubKey(p.host.ID()))

	if err != nil {
		panic("Failed to get public key for sender from local peer store.")
	}

	return &p2p.MessageData{ClientVersion: clientVersion,
		NodeId:     p.host.ID().String(),
		NodePubKey: nodePubKey,
		Timestamp:  time.Now().Unix(),
		Id:         messageId,
		Gossip:     gossip}
}

// helper method - writes a protobuf go data object to a network stream
// data: reference of protobuf go data object to send (not the object itself)
// s: network stream to write the data to
func (p *PingProtocol) sendProtoMessage(id peer.ID, pid protocol.ID, data proto.Message) bool {
	s, err := p.host.NewStream(context.Background(), id, pid)
	if err != nil {
		log.Println(err)
		return false
	}
	defer s.Close()

	writer := ggio.NewFullWriter(s)
	err = writer.WriteMsg(data)
	if err != nil {
		log.Println(err)
		s.Reset()
		return false
	}
	return true
}

// pattern: /protocol-name/request-or-response-message/version
const pingRequest = "/ping/pingreq/0.0.1"
const pingResponse = "/ping/pingresp/0.0.1"

// PingProtocol type
type PingProtocol struct {
	host     host.Host
	mu       sync.Mutex
	requests map[string]*p2p.PingRequest // used to access request data from response handlers. Protected by mu
	done     chan bool                   // only for demo purposes to stop main from terminating
}

func NewPingProtocol(host host.Host, done chan bool) *PingProtocol {
	p := &PingProtocol{host: host, requests: make(map[string]*p2p.PingRequest), done: done}
	p.host.SetStreamHandler(pingRequest, p.onPingRequest)
	p.host.SetStreamHandler(pingResponse, p.onPingResponse)
	return p
}

// remote peer requests handler
func (p *PingProtocol) onPingRequest(s network.Stream) {

	// get request data
	data := &p2p.PingRequest{}
	buf, err := io.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Println(err)
		return
	}
	s.Close()

	// unmarshal it
	err = proto.Unmarshal(buf, data)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("%s: Received ping request from %s. Message: %s", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.Message)

	valid := p.authenticateMessage(data, data.MessageData)

	if !valid {
		log.Println("Failed to authenticate message")
		return
	}

	// generate response message
	log.Printf("%s: Sending ping response to %s. Message id: %s...", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.MessageData.Id)

	resp := &p2p.PingResponse{MessageData: p.NewMessageData(data.MessageData.Id, false),
		Message: fmt.Sprintf("Ping response from %s", p.host.ID())}

	// sign the data
	signature, err := p.signProtoMessage(resp)
	if err != nil {
		log.Println("failed to sign response")
		return
	}

	// add the signature to the message
	resp.MessageData.Sign = signature

	// send the response
	ok := p.sendProtoMessage(s.Conn().RemotePeer(), pingResponse, resp)

	if ok {
		log.Printf("%s: Ping response to %s sent.", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	}
	p.done <- true
}

// remote ping response handler
func (p *PingProtocol) onPingResponse(s network.Stream) {
	data := &p2p.PingResponse{}
	buf, err := io.ReadAll(s)
	if err != nil {
		s.Reset()
		log.Println(err)
		return
	}
	s.Close()

	// unmarshal it
	err = proto.Unmarshal(buf, data)
	if err != nil {
		log.Println(err)
		return
	}

	valid := p.authenticateMessage(data, data.MessageData)

	if !valid {
		log.Println("Failed to authenticate message")
		return
	}

	// locate request data and remove it if found
	p.mu.Lock()
	_, ok := p.requests[data.MessageData.Id]
	if ok {
		// remove request from map as we have processed it here
		delete(p.requests, data.MessageData.Id)
	} else {
		log.Println("Failed to locate request data boject for response")
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	log.Printf("%s: Received ping response from %s. Message id:%s. Message: %s.", s.Conn().LocalPeer(), s.Conn().RemotePeer(), data.MessageData.Id, data.Message)
	p.done <- true
}

// Ping sends a ping request to a peer
func (p *PingProtocol) Ping(target peer.ID) bool {
	log.Printf("%s: Sending ping to: %s....", p.host.ID(), target)

	// create message data
	req := &p2p.PingRequest{
		MessageData: p.NewMessageData(uuid.New().String(), false),
		Message:     fmt.Sprintf("Ping from %s", p.host.ID()),
	}

	// Sign the data
	signature, err := p.signProtoMessage(req)
	if err != nil {
		log.Println("Failed to sign ping request:", err)
		return false
	}

	// add the signature to the message
	req.MessageData.Sign = signature

	// store ref request so response handler has access to it
	p.mu.Lock()
	p.requests[req.MessageData.Id] = req
	p.mu.Unlock()

	// Send the ping request using the Ping Protocol
	ok := p.sendProtoMessage(target, pingRequest, req)
	if !ok {
		return false
	}

	log.Printf("%s: Ping to: %s was sent. Message ID: %s, Message: %s", p.host.ID(), target, req.MessageData.Id, req.Message)
	return true
}
