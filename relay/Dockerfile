FROM golang:1.23

WORKDIR /usr/src/app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o relay_server .

#removed expose for host

#this is overwrite by compose
CMD ["./relay_server", "-port=1237","-key=0"]