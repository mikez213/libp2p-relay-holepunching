FROM golang:1.23

WORKDIR /usr/src/app

COPY mobile_client/go.mod mobile_client/go.sum ./mobile_client/
# COPY mobile_client/go.mod mobile_client/go.sum ./mobile_client/

COPY ping ./mobile_client/ping/

COPY shared ./mobile_client/shared/

WORKDIR /usr/src/app/mobile_client

RUN go mod download

COPY mobile_client/. .

RUN go build -o mobile_client .

CMD ["./mobile_client", '-bootstrap="runthisindocker or /ip4/10.0.20.118/tcp/1234/p2p/12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n"']
