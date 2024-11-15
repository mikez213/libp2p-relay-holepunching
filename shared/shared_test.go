package cmn

import (
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
)

func decodePeerID(idStr string) peer.ID {
	pid, err := peer.Decode(idStr)
	if err != nil {
		panic(fmt.Sprintf("Invalid peer ID: %s", idStr))
	}
	return pid
}
func TestIsBootstrapPeer(t *testing.T) {
	type args struct {
		peerID peer.ID
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Peer is in BootstrapPeerIDs",
			args: args{
				peerID: decodePeerID("12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n"),
			},
			want: true,
		},
		{
			name: "Peer is not in BootstrapPeerIDs",
			args: args{
				peerID: decodePeerID("12D3KooWQaZ9Ppi8A2hcEspJhewfPqKjtXu4vx7FQPaUGnHXWpNL"),
			},
			want: false,
		},
	}

	BootstrapPeerIDs = []peer.ID{
		decodePeerID("12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n"),
		decodePeerID("12D3KooWBnext3VBZZuBwGn3YahAZjf49oqYckfx64VpzH6dyU1p"),
		decodePeerID("12D3KooWDKYjXDDgSGzhEYWYtDvfP9pMtGNY1vnAwRsSp2CwCWHL"),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBootstrapPeer(tt.args.peerID); got != tt.want {
				t.Errorf("IsBootstrapPeer() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRelayIdentity(t *testing.T) {
	type args struct {
		keyIndex int
	}
	tests := []struct {
		name    string
		args    args
		wantNil bool
		wantErr bool
	}{
		{
			name:    "Valid key index 0",
			args:    args{keyIndex: 0},
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "Valid key index in middle",
			args:    args{keyIndex: len(RelayerPrivateKeys) / 2},
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "Valid key index last",
			args:    args{keyIndex: len(RelayerPrivateKeys) - 1},
			wantNil: false,
			wantErr: false,
		},
		{
			name:    "Invalid key index negative",
			args:    args{keyIndex: -1},
			wantNil: true,
			wantErr: true,
		},
		{
			name:    "Invalid key index beyond range",
			args:    args{keyIndex: len(RelayerPrivateKeys)},
			wantNil: true,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RelayIdentity(tt.args.keyIndex)
			if (err != nil) != tt.wantErr {
				t.Errorf("RelayIdentity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (got == nil) != tt.wantNil {
				t.Errorf("RelayIdentity() got = %v, wantNil %v", got, tt.wantNil)
			}
		})
	}
}

func TestParseBootstrap(t *testing.T) {
	type args struct {
		bootstrapAddrs []string
	}
	tests := []struct {
		name string
		args args
		want int // Number of expected valid peers
	}{
		{
			name: "Valid bootstrap addresses",
			args: args{
				bootstrapAddrs: []string{
					"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWQaZ9Ppi8A2hcEspJhewfPqKjtXu4vx7FQPaUGnHXWpNL",
					"/ip4/127.0.0.1/tcp/4002/p2p/12D3KooWJuteouY1d5SYFcAUAYDVPjFD8MUBgqsdjZfBkAecCS2Y",
				},
			},
			want: 2,
		},
		{
			name: "Invalid bootstrap addresses",
			args: args{
				bootstrapAddrs: []string{
					"/invalid/address",
				},
			},
			want: 0,
		},
		{
			name: "Mixed valid and invalid addresses",
			args: args{
				bootstrapAddrs: []string{
					"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWQaZ9Ppi8A2hcEspJhewfPqKjtXu4vx7FQPaUGnHXWpNL",
					"/invalid/address",
				},
			},
			want: 1,
		},
		{
			name: "Empty address list",
			args: args{
				bootstrapAddrs: []string{},
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseBootstrap(tt.args.bootstrapAddrs)
			if len(got) != tt.want {
				t.Errorf("ParseBootstrap() = %v peers, want %v peers", len(got), tt.want)
			}
		})
	}
}
