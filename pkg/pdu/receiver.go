package pdu

import (
	"fmt"
	_type "github.com/zyra/bacnet-2/pkg/type"
	"github.com/zyra/bacnet-2/pkg/util"
	"net"
	"time"
)

type Receiver struct {
	*Pdu
	Timeout        time.Duration
	Done           chan int
	Once           bool
	addr           *net.UDPAddr
	conn           *net.UDPConn
	tx             chan Response
	rx             chan int
	suppressErrors bool
}

func NewPduReceiver(source *net.IP, port uint16) *Receiver {
	d := &Receiver{
		Pdu: NewPdu(),
	}
	d.Source = source
	d.SourcePort = port
	d.addr = util.GetUdpAddr(d.Source, d.SourcePort)
	d.rx = make(chan int)
	return d
}

func (d *Receiver) Receive(c chan Response) {
	// Start UDP listener
	udpAddr := util.GetUdpAddr(d.Target, d.TargetPort)
	conn, err := net.ListenUDP("udp", udpAddr)
	d.tx = c

	if err != nil {
		fmt.Println("Error starting UDP server", err)
		return
	}

	d.conn = conn

	if d.Timeout == 0 {
		// Set default timeout to 10 seconds
		d.Timeout = time.Second * 10
	}

	if err = conn.SetReadDeadline(time.Now().Add(d.Timeout)); err != nil {
		fmt.Println("Error setting read deadline", err)
		return
	}

	if !d.Once {
		d.Done = make(chan int, 1)
	}

	timeout := time.After(d.Timeout)

	// Start reading packets
	go func(d *Receiver) {
		defer d.close()
		go d.receive()
	Loop:
		for {
			select {
			case <-d.rx:
				if d.Once {
					break Loop
				} else {
					go d.receive()
					continue
				}
			case <-timeout:
				d.suppressErrors = true
				break Loop
			}
		}
		fmt.Println("Done receiving")

		if !d.Once {
			d.Done <- 1
		}
	}(d)
}

func (d *Receiver) close() {
	// Close UDP connection
	if err := d.conn.Close(); err != nil {
		fmt.Println("Error stopping UDP server", err)
	}
}

func (d *Receiver) receive() {
	b := make([]byte, _type.MAX_MPDU)
	if l, addr, err := d.conn.ReadFromUDP(b[:]); err != nil {
		if !d.suppressErrors {
			fmt.Println("Error reading from UDP", err)
		}
	} else {
		go d.emit(b, addr)
		d.rx <- l
	}
}

func (d *Receiver) emit(data []byte, address *net.UDPAddr) {
	fmt.Println("Received UDP message from: ", address)
	pdu := NewPduResponse(data)
	pdu.Sender = address

	d.tx <- *pdu
}
