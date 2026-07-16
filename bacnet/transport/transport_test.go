package transport

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestMemoryNetworkUnicastAndPayloadOwnership(t *testing.T) {
	network := NewMemoryNetwork()
	sender, err := network.Listen(NewEndpoint(net.IPv4(127, 0, 0, 1), 47808))
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := network.Listen(NewEndpoint(net.IPv4(127, 0, 0, 2), 47808))
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte{1, 2, 3}
	if err := sender.Write(context.Background(), receiver.LocalEndpoint(), payload); err != nil {
		t.Fatal(err)
	}
	payload[0] = 9

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	datagram, err := receiver.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if datagram.Payload[0] != 1 {
		t.Fatalf("received payload was modified: %v", datagram.Payload)
	}
}

func TestMemoryNetworkBroadcast(t *testing.T) {
	network := NewMemoryNetwork()
	sender, _ := network.Listen(NewEndpoint(net.IPv4(127, 0, 0, 1), 47808))
	receiver1, _ := network.Listen(NewEndpoint(net.IPv4(127, 0, 0, 2), 47808))
	receiver2, _ := network.Listen(NewEndpoint(net.IPv4(127, 0, 0, 3), 47808))

	destination := NewEndpoint(net.IPv4bcast, 47808)
	if err := sender.Write(context.Background(), destination, []byte{42}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := receiver1.Read(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := receiver2.Read(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryConnCloseIsIdempotent(t *testing.T) {
	conn, err := NewMemoryNetwork().Listen(NewEndpoint(net.IPv4(127, 0, 0, 1), 47808))
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = conn.Read(context.Background())
	if err != ErrClosed {
		t.Fatalf("read error = %v, want ErrClosed", err)
	}
}

func TestUDPTransportRoundTrip(t *testing.T) {
	left, err := ListenUDP(NewEndpoint(net.IPv4(127, 0, 0, 1), 0))
	if err != nil {
		t.Fatal(err)
	}
	defer left.Close()
	right, err := ListenUDP(NewEndpoint(net.IPv4(127, 0, 0, 1), 0))
	if err != nil {
		t.Fatal(err)
	}
	defer right.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := left.Write(ctx, right.LocalEndpoint(), []byte("bacnet")); err != nil {
		t.Fatal(err)
	}
	datagram, err := right.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(datagram.Payload) != "bacnet" {
		t.Fatalf("payload = %q", datagram.Payload)
	}
}
