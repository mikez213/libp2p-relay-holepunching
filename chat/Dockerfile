FROM golang:1.23

WORKDIR /usr/src/app

COPY go.mod go.sum main.go ./

RUN go mod download && go mod verify

CMD ["go", "run", "main.go"]
