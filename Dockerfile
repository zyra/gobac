FROM golang:alpine

RUN mkdir -p /go/src/github.com/zyra/gobac
COPY . /go/src/github.com/zyra/gobac/