FROM golang:alpine

WORKDIR /root/gobac
COPY . .
RUN go build -mod vendor -o /go/bin/gobac cmd/gobac/main.go

ENTRYPOINT ["gobac"]