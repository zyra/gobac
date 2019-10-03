FROM golang:1.12-alpine

WORKDIR /root/gobac
COPY . .
RUN go build -mod vendor -o /go/bin/gobac cmd/gobac/main.go

ENTRYPOINT ["gobac"]