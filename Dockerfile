FROM golang:alpine

RUN mkdir -p /go/src/github.com/zyra/gobac
COPY bacnet /go/src/github.com/zyra/gobac/