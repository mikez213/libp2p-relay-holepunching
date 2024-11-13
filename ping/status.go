package customprotocol

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	p2p "github.com/mikez213/libp2p-relay-holepunching/ping/pb"
	proto "google.golang.org/protobuf/proto"
)

type StatusRequestHandler struct {
	protocol *PingProtocol
}

func (h *StatusRequestHandler) Handle(s network.Stream, from peer.ID, data []byte) error {
	var req p2p.StatusRequest
	if err := proto.Unmarshal(data, &req); err != nil {
		log.Errorf("Failed to unmarshal StatusRequest: %v", err)
		s.Reset()
		return err
	}

	log.Infof("Received StatusRequest from %s: ProjectID=%s, DevID=%s, APIKey=%s",
		from, req.Id.ProjectId, req.Id.DevId, req.Id.ApiKey)

	// logic here
	statusMessage := "unknown"

	if isStreaming {
		statusMessage = "Stream is active"
	} else {
		statusMessage = "Stream is inactive"
	}

	resp := &p2p.StatusResponse{
		IsStreaming:   isStreaming,
		StatusMessage: statusMessage,
	}

	ok := h.protocol.sendProtoMessage(s.Conn().RemotePeer(), statusResponse, resp)

	if ok {
		log.Infof("%s: StatusResponse sent to %s.", h.protocol.host.ID().String(), from.String())
	} else {
		err := fmt.Errorf("%s: Error in sending StatusResponse to %s", h.protocol.host.ID().String(), from.String())
		return err
	}

	log.Infof("Sent StatusResponse to %s: IsStreaming=%v, StatusMessage=%s", from, isStreaming, statusMessage)
	h.protocol.done <- true
	return nil
}

type StatusResponseHandler struct {
	protocol *PingProtocol
}

func (h *StatusResponseHandler) Handle(s network.Stream, from peer.ID, data []byte) error {
	var resp p2p.StatusResponse
	if err := proto.Unmarshal(data, &resp); err != nil {
		log.Errorf("Failed to unmarshal StatusResponse: %v", err)
		s.Reset()
		return err
	}

	log.Infof("Received StatusResponse from %s: IsStreaming=%v, StatusMessage=%s",
		from, resp.IsStreaming, resp.StatusMessage)
	h.protocol.done <- true
	return nil
}
