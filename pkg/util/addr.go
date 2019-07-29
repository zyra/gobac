package util

import "net"

func GetUdpAddr(ip *net.IP, port uint16) *net.UDPAddr {
	return &net.UDPAddr{
		IP:   *ip,
		Port: int(port),
	}
}
