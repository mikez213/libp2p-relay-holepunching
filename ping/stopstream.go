// stopstream.go
package customprotocol

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	p2p "github.com/mikez213/libp2p-relay-holepunching/ping/pb"
	proto "google.golang.org/protobuf/proto"
)

type StopStreamRequestHandler struct {
	protocol *PingProtocol
}

func (h *StopStreamRequestHandler) Handle(s network.Stream, from peer.ID, data []byte) error {
	var req p2p.StopStreamRequest
	if err := proto.Unmarshal(data, &req); err != nil {
		log.Errorf("Failed to unmarshal StopStreamRequest: %v", err)
		s.Reset()
		return err
	}

	log.Infof("Received StopStreamRequest from %s: ProjectID=%s, DevID=%s, APIKey=%s",
		from, req.Id.ProjectId, req.Id.DevId, req.Id.ApiKey)

	//logic here
	statusMessage := "unknown"

	if !isStreaming {
		log.Info("WE WERE NOT STREAMING")
		statusMessage = "NOT_STREAMING_PREVIOUSLY" //replace str with proto value
	} else {
		log.Warn("STOPPED ACTIVE STREAM")
		isStreaming = false
		statusMessage = "STREAM_STOPPED"
	}
	resp := &p2p.StopStreamResponse{
		Id: &p2p.Id{ProjectId: req.Id.ProjectId, DevId: req.Id.DevId, ApiKey: req.Id.ApiKey},
	}

	ok := h.protocol.sendProtoMessage(s.Conn().RemotePeer(), stopStreamResponse, resp)

	if ok {
		log.Infof("%s: StopStreamResponse sent to %s.", h.protocol.host.ID().String(), from.String())
	} else {
		err := fmt.Errorf("%s: Error in sending StopStreamResponse to %s", h.protocol.host.ID().String(), from.String())
		return err
	}

	log.Infof("Sent StopStreamResponse to %s", from, isStreaming, statusMessage)
	h.protocol.done <- true
	return nil
}

type StopStreamResponseHandler struct {
	protocol *PingProtocol
}

func (h *StopStreamResponseHandler) Handle(s network.Stream, from peer.ID, data []byte) error {
	var resp p2p.StopStreamResponse
	if err := proto.Unmarshal(data, &resp); err != nil {
		log.Errorf("Failed to unmarshal StopStreamResponse: %v", err)
		s.Reset()
		return err
	}

	log.Infof("Received StopStreamResponse from %s", from)
	h.protocol.done <- true
	return nil
}
