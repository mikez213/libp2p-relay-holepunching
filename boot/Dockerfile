FROM golang:1.23

WORKDIR /usr/src/app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o bootstrap_server .

EXPOSE 1237 1238 1239

#this is overwrite by compose
CMD ["./bootstrap_server", "-port=1237","-key=0"]