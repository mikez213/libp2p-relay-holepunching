package customprotocol

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	p2p "github.com/mikez213/libp2p-relay-holepunching/ping/pb"
	proto "google.golang.org/protobuf/proto"
)

type PingRequestHandler struct {
	protocol *PingProtocol
}
type PingResponseHandler struct {
	protocol *PingProtocol
}

func (h *PingRequestHandler) Handle(s network.Stream, from peer.ID, data []byte) error {
	var req p2p.PingRequest
	if err := proto.Unmarshal(data, &req); err != nil {
		log.Errorf("Failed to unmarshal PingRequest: %v", err)
		s.Reset()
		return err
	}

	log.Infof("Received PingRequest from %s: %s", from, req.MessageData)

	resp := &p2p.PingResponse{
		MessageData: fmt.Sprintf("Response to %s", req.MessageData),
		Message:     fmt.Sprintf("Ping response from %s", h.protocol.host.ID()),
	}

	ok := h.protocol.sendProtoMessage(s.Conn().RemotePeer(), pingResponse, resp)

	if ok {
		log.Infof("%s: %T response to %s sent.", s.Conn().LocalPeer().String(), resp, s.Conn().RemotePeer().String())
	} else {
		log.Errorf("%s: Error in Ping response to %s", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
	}

	log.Infof("Sent PingResponse to %s: %s", from, resp.MessageData)
	h.protocol.done <- true
	return nil
}

func (h *PingResponseHandler) Handle(s network.Stream, from peer.ID, data []byte) error {
	var resp p2p.PingResponse
	if err := proto.Unmarshal(data, &resp); err != nil {
		log.Errorf("Failed to unmarshal PingResponse: %v", err)
		s.Reset()
		return err
	}

	log.Infof("Received PingResponse from %s: %s", from, resp.MessageData)
	h.protocol.done <- true
	return nil
}
