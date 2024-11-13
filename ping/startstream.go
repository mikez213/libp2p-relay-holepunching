package customprotocol

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	p2p "github.com/mikez213/libp2p-relay-holepunching/ping/pb"
	proto "google.golang.org/protobuf/proto"
)

type StartStreamRequestHandler struct {
	protocol *PingProtocol
}

func (h *StartStreamRequestHandler) Handle(s network.Stream, from peer.ID, data []byte) error {
	var req p2p.StartStreamRequest
	if err := proto.Unmarshal(data, &req); err != nil {
		log.Errorf("Failed to unmarshal StartStreamRequest: %v", err)
		s.Reset()
		return err
	}

	log.Infof("Received StartStreamRequest from %s: ProjectID=%s, DevID=%s, APIKey=%s, IssueNeed=%s, ConfigOptions=%v",
		from, req.Id.ProjectId, req.Id.DevId, req.Id.ApiKey, req.RequestIssueNeed, req.ConfigOptions)

	// TODO: logic!
	statusMessage := "unknown"

	if !isStreaming {
		log.Info("WE ARE NOW STREAMING")
		isStreaming = true
		statusMessage = "SUCCESS" //replace str with proto value
	} else {
		log.Warn("WE ARE ALREADY STRAMING")
		// isStreaming = true
		statusMessage = "ALREADY_STREAMING"
	}

	resp := &p2p.StartStreamResponse{
		Id:            &p2p.Id{ProjectId: req.Id.ProjectId, DevId: req.Id.DevId, ApiKey: req.Id.ApiKey},
		IsStreaming:   isStreaming,
		StatusMessage: statusMessage,
	}

	ok := h.protocol.sendProtoMessage(s.Conn().RemotePeer(), startStreamResponse, resp)

	if ok {
		log.Infof("%s: %T response to %s sent.", s.Conn().LocalPeer().String(), resp, s.Conn().RemotePeer().String())
	} else {
		err := fmt.Errorf("%s: Error in sending response to %s", s.Conn().LocalPeer().String(), s.Conn().RemotePeer().String())
		return err
	}

	log.Infof("Sent StartStreamResponse to %s: IsStreaming=%v, StatusMessage=%s", from, isStreaming, statusMessage)
	h.protocol.done <- true
	return nil
}

type StartStreamResponseHandler struct {
	protocol *PingProtocol
}

func (h *StartStreamResponseHandler) Handle(s network.Stream, from peer.ID, data []byte) error {
	var resp p2p.StartStreamResponse
	if err := proto.Unmarshal(data, &resp); err != nil {
		log.Errorf("Failed to unmarshal StartStreamResponse: %v", err)
		s.Reset()
		return err
	}

	log.Infof("Received StartStreamResponse from %s: IsStreaming=%v, StatusMessage=%s",
		from, resp.IsStreaming, resp.StatusMessage)
	h.protocol.done <- true
	return nil
}
