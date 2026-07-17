package simulator

import (
	"context"
	"net"
	"testing"

	"github.com/zyra/gobac/v2/bacnet"
	"github.com/zyra/gobac/v2/bacnet/pdu"
	"github.com/zyra/gobac/v2/bacnet/responder"
	"github.com/zyra/gobac/v2/bacnet/transport"
	"github.com/zyra/gobac/v2/bacnet/types"
)

func timeSyncTestDevice() *Device {
	deviceID := ObjectID{Type: uint16(types.ObjectTypeDevice), Instance: 70000}
	return &Device{
		ID:   70000,
		Name: "Test Device",
		Objects: map[ObjectID]*Object{
			deviceID: {
				ID:         deviceID,
				Name:       "Test Device",
				Properties: map[uint32]*Property{},
			},
		},
	}
}

func timeSyncTestRequest(t *testing.T, sync *pdu.TimeSyncPdu) *responder.Request {
	t.Helper()
	payload, err := sync.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	packet := bacnet.NewRequest()
	packet.Apdu.Payload = payload
	return &responder.Request{
		Packet:      packet,
		Destination: transport.NewEndpoint(net.IPv4bcast, 47808),
	}
}

func TestApplicationHandleTimeSyncSetsLocalDateAndTime(t *testing.T) {
	device := timeSyncTestDevice()
	deviceID := ObjectID{Type: uint16(types.ObjectTypeDevice), Instance: device.ID}
	application := NewApplication(device, RealClock{})

	if _, err := device.ReadProperty(deviceID, uint32(types.PropertyLocalTime), nil); err != ErrUnknownProperty {
		t.Fatalf("pre-sync Local_Time read error = %v, want %v", err, ErrUnknownProperty)
	}
	if _, err := device.ReadProperty(deviceID, uint32(types.PropertyLocalDate), nil); err != ErrUnknownProperty {
		t.Fatalf("pre-sync Local_Date read error = %v, want %v", err, ErrUnknownProperty)
	}

	sync := &pdu.TimeSyncPdu{
		Date: types.Date{Year: 2026, Month: 7, Day: 17, Weekday: types.WeekdayFriday},
		Time: types.Time{Hour: 14, Min: 30, Sec: 15, Hundredths: 25},
	}
	request := timeSyncTestRequest(t, sync)
	responses, err := application.handleTimeSync(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(responses) != 0 {
		t.Fatalf("responses = %+v, want none", responses)
	}

	timeValues, err := device.ReadProperty(deviceID, uint32(types.PropertyLocalTime), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(timeValues) != 1 || timeValues[0].Tag != types.TagTime || timeValues[0].Value != sync.Time {
		t.Fatalf("Local_Time = %+v, want Tag=%d Value=%+v", timeValues, types.TagTime, sync.Time)
	}

	dateValues, err := device.ReadProperty(deviceID, uint32(types.PropertyLocalDate), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(dateValues) != 1 || dateValues[0].Tag != types.TagDate || dateValues[0].Value != sync.Date {
		t.Fatalf("Local_Date = %+v, want Tag=%d Value=%+v", dateValues, types.TagDate, sync.Date)
	}
}

func TestApplicationHandleUtcTimeSyncSetsLocalDateAndTime(t *testing.T) {
	device := timeSyncTestDevice()
	deviceID := ObjectID{Type: uint16(types.ObjectTypeDevice), Instance: device.ID}
	application := NewApplication(device, RealClock{})

	sync := &pdu.TimeSyncPdu{
		Date: types.Date{Year: 2026, Month: 7, Day: 19, Weekday: types.WeekdaySunday},
		Time: types.Time{Hour: 8, Min: 0, Sec: 0, Hundredths: 0},
	}
	request := timeSyncTestRequest(t, sync)
	if _, err := application.handleTimeSync(context.Background(), request); err != nil {
		t.Fatal(err)
	}

	timeValues, err := device.ReadProperty(deviceID, uint32(types.PropertyLocalTime), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(timeValues) != 1 || timeValues[0].Value != sync.Time {
		t.Fatalf("Local_Time = %+v, want %+v", timeValues, sync.Time)
	}
}

func TestApplicationHandleTimeSyncIgnoresMalformedPayload(t *testing.T) {
	device := timeSyncTestDevice()
	deviceID := ObjectID{Type: uint16(types.ObjectTypeDevice), Instance: device.ID}
	application := NewApplication(device, RealClock{})

	packet := bacnet.NewRequest()
	packet.Apdu.Payload = []byte{0xa4, 0x7e, 0x07, 0x11, 0x05} // date only, missing time
	request := &responder.Request{Packet: packet, Destination: transport.NewEndpoint(net.IPv4bcast, 47808)}

	responses, err := application.handleTimeSync(context.Background(), request)
	if err != nil || len(responses) != 0 {
		t.Fatalf("malformed payload response = %+v, %v, want nil, nil", responses, err)
	}
	if _, err := device.ReadProperty(deviceID, uint32(types.PropertyLocalTime), nil); err != ErrUnknownProperty {
		t.Fatalf("Local_Time read error after malformed payload = %v, want %v", err, ErrUnknownProperty)
	}
}

func TestApplicationRegistersTimeSyncHandlers(t *testing.T) {
	device := timeSyncTestDevice()
	deviceID := ObjectID{Type: uint16(types.ObjectTypeDevice), Instance: device.ID}
	application := NewApplication(device, RealClock{})
	server := responder.NewServer()
	application.Register(server)

	sync := &pdu.TimeSyncPdu{
		Date: types.Date{Year: 2026, Month: 7, Day: 17, Weekday: types.WeekdayFriday},
		Time: types.Time{Hour: 14, Min: 30, Sec: 15, Hundredths: 25},
	}
	payload, err := sync.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	datagramBytes := applicationPacket(t, types.PduTypeUnconfirmedServiceRequest, types.UnconfirmedServiceUtcTimeSynchronization, 0, payload, true)

	network := transport.NewMemoryNetwork()
	deviceEndpoint := transport.NewEndpoint(net.IPv4(192, 0, 2, 20), 47808)
	deviceConn, err := network.Listen(deviceEndpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer deviceConn.Close()

	datagram := transport.Datagram{
		Payload:     datagramBytes,
		Source:      transport.NewEndpoint(net.IPv4(192, 0, 2, 10), 47808),
		Destination: transport.NewEndpoint(net.IPv4bcast, 47808),
	}
	if err := server.ServeDatagram(context.Background(), deviceConn, datagram); err != nil {
		t.Fatal(err)
	}

	timeValues, err := device.ReadProperty(deviceID, uint32(types.PropertyLocalTime), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(timeValues) != 1 || timeValues[0].Value != sync.Time {
		t.Fatalf("Local_Time after UTCTimeSynchronization dispatch = %+v, want %+v", timeValues, sync.Time)
	}
}
