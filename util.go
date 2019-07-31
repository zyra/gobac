package gobac

import (
	"errors"
	"math"
	"net"
	"time"
)

type networkSet struct {
	InterfaceName string
	Interface     *net.Interface
	IPv4          *net.IP
	BroadcastIPv4 *net.IP
}

func getNetworkSet(ifname string) (*networkSet, error) {
	iface, err := net.InterfaceByName(ifname)

	if err != nil {
		return nil, err
	}

	ns := &networkSet{
		InterfaceName: ifname,
		Interface:     iface,
	}

	var sourceIP *net.IP
	var broadcastIp *net.IP

	addrs, err := iface.Addrs()

	if err != nil {
		return nil, err
	}

	// Get local ip
	for _, addr := range addrs {
		ip, nt, err := net.ParseCIDR(addr.String())

		if err != nil {
			continue
		}

		ip = ip.To4()

		if ip != nil {
			broadcastIp = &net.IP{
				ip[0] | ^ nt.Mask[0],
				ip[1] | ^ nt.Mask[1],
				ip[2] | ^ nt.Mask[2],
				ip[3] | ^ nt.Mask[3],
			}

			sourceIP = &ip
			break
		}
	}

	if sourceIP == nil {
		return nil, errors.New("no valid IPv4 was found")
	}

	if broadcastIp == nil {
		return nil, errors.New("no valid broadcast IPv4 was found")
	}

	ns.IPv4 = sourceIP
	ns.BroadcastIPv4 = broadcastIp

	return ns, nil
}

func getChanHandler() (<-chan *Response, responseHandler) {
	c := make(chan *Response)
	return c, func(response *Response) {
		c <- response
	}
}

func getChanHandlerWithTimeout(t time.Duration) (tc <-chan time.Time, c <-chan *Response, handler responseHandler) {
	c, handler = getChanHandler()
	tc = time.After(t)
	return tc, c, handler
}

func getUdpAddr(ip *net.IP, port uint16) *net.UDPAddr {
	return &net.UDPAddr{
		IP:   *ip,
		Port: int(port),
	}
}

func getSignedLen(val int) uint32 {
	if val <= math.MaxInt8 && val >= math.MinInt16 {
		return 1
	}

	if val <= math.MaxInt16 && val >= math.MinInt16 {
		return 2
	}

	if val <= 8388607 && val >= -8388607 {
		return 3
	}

	return 4
}

func getUnsignedLen(val uint) uint32 {
	if val <= math.MaxUint8 {
		return 1
	} else if val <= math.MaxUint16 {
		return 2
	} else if val <= 1<<24-1 {
		return 3
	} else {
		return 4
	}
}
