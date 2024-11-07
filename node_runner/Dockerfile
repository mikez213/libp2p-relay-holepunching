FROM golang:1.23

WORKDIR /usr/src/app

COPY node_runner/go.mod node_runner/go.sum ./node_runner/

COPY ping ./node_runner/ping/

WORKDIR /usr/src/app/node_runner

RUN go mod download

COPY node_runner/. .

RUN go build -o node_runner .

CMD ["./node_runner", '-bootstrap="runthisindocker or /ip4/10.0.20.118/tcp/1234/p2p/12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n"']
