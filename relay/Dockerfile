FROM golang:1.23

WORKDIR /usr/src/app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o relay_node .

#removed expose for host
EXPOSE 1237 1238 1239 1240 1241 1242

#this is overwrite by compose
CMD ["./relay_node", "-port=1240","-key=3"]