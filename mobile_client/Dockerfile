FROM golang:1.23

WORKDIR /usr/src/app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o mobile_client .

# EXPOSE 1234 1237 1238 1239

#this is overwrite by compose
CMD ["./mobile_client", '-bootstrap="runthisindocker or /ip4/10.0.20.118/tcp/1234/p2p/12D3KooWLr1gYejUTeriAsSu6roR2aQ423G3Q4fFTqzqSwTsMz9n"']