# GoBAC
BACnet protocol implementation in Go

[![Build Status](https://drone.zyra.ca/api/badges/zyra/gobac/status.svg)](https://drone.zyra.ca/zyra/gobac)

### Build CLI
```sh
# build locally with go installed
go build -mod vendor -o gobac cmd/gobac/main.go

# build with docker
docker build -t gobac . 
```