# Peer discovery and streaming with LibP2P

## Overview

#### Nodes
Mobile Client: initiates and looks for a node runner to get a stream from
Node Runner: will accept connections and streams content
Relay: Helps discovery and and connections over private/NAT networks
Bootstrap: Multiple known static nodes that can always be discovered for initialization

### Requirements
```
Docker
Docker Compose
Go
```

## Usage

```sh
> docker-compose up --build
```

### Protobuf Generation

TODO: Refactor to remove the replacement due to docker

If changes are made to the protocol it must be rebuilt.

To regenerate the protobuf:
From the `ping` directory run the following:

```sh
> cd ping
> protoc --go_out=. --go_opt=paths=source_relative pb/p2p.proto
```

Then from `node_runner/go.mod` and `mobile_client/go.mod` replace

`replace github.com/mikez213/libp2p-relay-holepunching/ping => ./ping` 
with 
`replace github.com/mikez213/libp2p-relay-holepunching/ping => ../ping`

then run in each folder run this command:
```sh
> go mod tidy
```

Undo the replace and then you will be able to build with ```docker-compose up --build```
For linting you may keep it as `../ping` however it will not build

## Author
@mikez213
