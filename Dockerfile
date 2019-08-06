FROM golang:alpine

RUN mkdir -p /go/src/github.com/zyra/gobac
COPY . /go/src/github.com/zyra/gobac/
WORKDIR /go/src/github.com/zyra/gobac/
RUN go build -o /go/bin/gobac cmd/gobac/main.go

ENTRYPOINT ["gobac"]