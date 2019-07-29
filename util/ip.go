package util

import (
	"errors"
	"fmt"
	"net"
)

type IPHelper struct {
	Ifname        string
	Interface     *net.Interface
	IPv4          *net.IP
	BroadcastIPv4 *net.IP
}

func NewIPHelper(ifname string) (*IPHelper, error) {
	iface, err := net.InterfaceByName(ifname)

	if err != nil {
		return nil, err
	}

	helper := &IPHelper{
		Ifname:    ifname,
		Interface: iface,
	}

	if err = helper.resolveIPv4(); err != nil {
		return nil, err
	}

	return helper, nil
}

func (h *IPHelper) resolveIPv4() error {
	var sourceIP *net.IP
	var broadcastIp *net.IP

	addrs, err := h.Interface.Addrs()

	if err != nil {
		return err
	}

	// Get local ip
	for _, addr := range addrs {
		ip, nt, err := net.ParseCIDR(addr.String())

		if err != nil {
			continue
		}

		ip = ip.To4()

		if ip != nil {
			fmt.Println(nt)

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
		return errors.New("no valid IPv4 was found")
	}

	if broadcastIp == nil {
		return errors.New("no valid broadcast IPv4 was found")
	}

	h.IPv4 = sourceIP
	h.BroadcastIPv4 = broadcastIp

	return nil
}