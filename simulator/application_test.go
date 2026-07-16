package simulator

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/zyra/gobac/bacnet"
	"github.com/zyra/gobac/bacnet/pdu"
	"github.com/zyra/gobac/bacnet/responder"
	"github.com/zyra/gobac/bacnet/transport"
	"github.com/zyra/gobac/bacnet/types"
)

func TestApplicationServicesOverMemoryNetwork(t *testing.T) {
	device := applicationTestDevice()
	application := NewApplication(device, RealClock{})
	server := responder.NewServer()
	application.Register(server)

	network := transport.NewMemoryNetwork()
	deviceEndpoint := transport.NewEndpoint(net.IPv4(192, 0, 2, 20), 47808)
	clientEndpoint := transport.NewEndpoint(net.IPv4(192, 0, 2, 10), 47808)
	deviceConn, err := network.Listen(deviceEndpoint)
	if err != nil {
		t.Fatal(err)
	}
	clientConn, err := network.Listen(clientEndpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer deviceConn.Close()
	defer clientConn.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.Serve(ctx, deviceConn)

	whoIs := applicationPacket(t, types.PduTypeUnconfirmedServiceRequest, types.UnconfirmedServiceWhoIs, 0, nil, true)
	if err := clientConn.Write(ctx, transport.NewEndpoint(net.IPv4bcast, 47808), whoIs); err != nil {
		t.Fatal(err)
	}
	iAm := readApplicationResponse(t, clientConn)
	if iAm.Apdu.PduType != types.PduTypeUnconfirmedServiceRequest || iAm.Apdu.ServiceChoice != types.UnconfirmedServiceIAm {
		t.Fatalf("unexpected I-Am APDU: %+v", iAm.Apdu)
	}
	discovered := iAm.ResponseData().(*types.Device)
	if discovered.DeviceInstance != device.ID {
		t.Fatalf("discovered device = %d", discovered.DeviceInstance)
	}
	iAm.Release()

	object := ObjectID{Type: uint16(types.ObjectTypeAnalogValue), Instance: 1}
	requestObject, err := toBACnetObjectID(object)
	if err != nil {
		t.Fatal(err)
	}
	readPayload, err := (&types.Property{ObjectId: requestObject, ID: types.PropertyPresentValue}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	read := applicationPacket(t, types.PduTypeConfirmedServiceRequest, types.ConfirmedServiceReadProperty, 10, readPayload, false)
	if err := clientConn.Write(ctx, deviceEndpoint, read); err != nil {
		t.Fatal(err)
	}
	readACK := readApplicationResponse(t, clientConn)
	if readACK.Apdu.PduType != types.PduTypeComplexAck || readACK.Apdu.InvokeID != 10 {
		t.Fatalf("unexpected ReadProperty ACK: %+v", readACK.Apdu)
	}
	property := readACK.ResponseData().(*pdu.ReadPropertyPdu).Property
	if len(property.Values) != 1 || property.Values[0].ReadAsFloat64() != 20.5 {
		t.Fatalf("read values = %+v", property.Values)
	}
	readACK.Release()

	writePayload, err := encodeReadPropertyResult(object, PropertyReference{ID: uint32(types.PropertyPresentValue)}, []Value{{Tag: types.TagReal, Value: float32(23.75)}})
	if err != nil {
		t.Fatal(err)
	}
	write := applicationPacket(t, types.PduTypeConfirmedServiceRequest, types.ConfirmedServiceWriteProperty, 11, writePayload, false)
	if err := clientConn.Write(ctx, deviceEndpoint, write); err != nil {
		t.Fatal(err)
	}
	writeACK := readApplicationResponse(t, clientConn)
	if writeACK.Apdu.PduType != types.PduTypeSimpleAck || writeACK.Apdu.InvokeID != 11 {
		t.Fatalf("unexpected WriteProperty ACK: %+v", writeACK.Apdu)
	}
	writeACK.Release()
	written, err := device.ReadProperty(object, uint32(types.PropertyPresentValue), nil)
	writtenValue, conversionErr := toPropertyValue(written[0])
	if err != nil || conversionErr != nil || writtenValue.ReadAsFloat64() != 23.75 {
		t.Fatalf("written value = %+v, %v", written, err)
	}

	rpmPayload := []byte{0x0c, 0x00, 0x80, 0x00, 0x01, 0x1e, 0x09, byte(types.PropertyAll), 0x1f}
	rpm := applicationPacket(t, types.PduTypeConfirmedServiceRequest, types.ConfirmedServiceReadPropertyMultiple, 12, rpmPayload, false)
	if err := clientConn.Write(ctx, deviceEndpoint, rpm); err != nil {
		t.Fatal(err)
	}
	rpmACK := readApplicationResponse(t, clientConn)
	if rpmACK.Apdu.PduType != types.PduTypeComplexAck || rpmACK.Apdu.ServiceChoice != types.ConfirmedServiceReadPropertyMultiple || len(rpmACK.Apdu.Payload) == 0 {
		t.Fatalf("unexpected RPM ACK: %+v", rpmACK.Apdu)
	}
	rpmACK.Release()

	subscribePayload := []byte{0x0b, 0x01, 0x00, 0x01, 0x1c, 0x00, 0x80, 0x00, 0x01, 0x29, 0x00, 0x39, 60}
	subscribe := applicationPacket(t, types.PduTypeConfirmedServiceRequest, types.ConfirmedServiceSubscribeCov, 13, subscribePayload, false)
	if err := clientConn.Write(ctx, deviceEndpoint, subscribe); err != nil {
		t.Fatal(err)
	}
	subscribeACK := readApplicationResponse(t, clientConn)
	if subscribeACK.Apdu.PduType != types.PduTypeSimpleAck || subscribeACK.Apdu.InvokeID != 13 {
		t.Fatalf("unexpected SubscribeCOV ACK: %+v", subscribeACK.Apdu)
	}
	subscribeACK.Release()
	initialCOV := readApplicationResponse(t, clientConn)
	if initialCOV.Apdu.PduType != types.PduTypeUnconfirmedServiceRequest || initialCOV.Apdu.ServiceChoice != types.UnconfirmedServiceCovNotification {
		t.Fatalf("unexpected initial COV notification: %+v", initialCOV.Apdu)
	}
	initialNotification := initialCOV.ResponseData().(*pdu.CovNotification)
	if initialNotification.ProcessIdentifier32 != 65537 || len(initialNotification.Properties) != 1 {
		t.Fatalf("initial COV notification = %+v", initialNotification)
	}
	initialCOV.Release()
	active := application.Subscriptions.Active()
	if len(active) != 1 || active[0].Key.ProcessID != 65537 || active[0].Confirmed {
		t.Fatalf("active subscriptions = %+v", active)
	}

	writePayload, err = encodeReadPropertyResult(object, PropertyReference{ID: uint32(types.PropertyPresentValue)}, []Value{{Tag: types.TagReal, Value: float32(24.5)}})
	if err != nil {
		t.Fatal(err)
	}
	write = applicationPacket(t, types.PduTypeConfirmedServiceRequest, types.ConfirmedServiceWriteProperty, 14, writePayload, false)
	if err := clientConn.Write(ctx, deviceEndpoint, write); err != nil {
		t.Fatal(err)
	}
	writeACK = readApplicationResponse(t, clientConn)
	if writeACK.Apdu.PduType != types.PduTypeSimpleAck {
		t.Fatalf("unexpected second WriteProperty ACK: %+v", writeACK.Apdu)
	}
	writeACK.Release()
	changedCOV := readApplicationResponse(t, clientConn)
	changedNotification := changedCOV.ResponseData().(*pdu.CovNotification)
	if changedNotification.Properties[0].Values[0].ReadAsFloat64() != 24.5 {
		t.Fatalf("changed COV values = %+v", changedNotification.Properties[0].Values)
	}
	changedCOV.Release()
}

func TestApplicationCOVIncrement(t *testing.T) {
	device := applicationTestDevice()
	objectID := ObjectID{Type: uint16(types.ObjectTypeAnalogValue), Instance: 1}
	device.Objects[objectID].Properties[uint32(types.PropertyPresentValue)].COVIncrement = 1
	application := NewApplication(device, RealClock{})
	previous := []Value{{Tag: types.TagReal, Value: float32(20)}}
	if application.covChanged(objectID, previous, []Value{{Tag: types.TagReal, Value: float32(20.5)}}) {
		t.Fatal("change below COV increment triggered")
	}
	if !application.covChanged(objectID, previous, []Value{{Tag: types.TagReal, Value: float32(21)}}) {
		t.Fatal("change at COV increment did not trigger")
	}
}

func TestApplicationWritePropertyPriorityErrors(t *testing.T) {
	device := applicationTestDevice()
	application := NewApplication(device, RealClock{})
	object := ObjectID{Type: uint16(types.ObjectTypeAnalogValue), Instance: 1}
	payload, err := encodeReadPropertyResult(object, PropertyReference{ID: uint32(types.PropertyPresentValue)}, []Value{{Tag: types.TagReal, Value: float32(21)}})
	if err != nil {
		t.Fatal(err)
	}

	request := bacnet.NewRequest()
	defer request.Release()
	request.Apdu.Payload = append(append([]byte(nil), payload...), 0x49, 17)
	responses, err := application.handleWriteProperty(context.Background(), &responder.Request{Packet: request})
	if err != nil || len(responses) != 1 || responses[0].PDUType != types.PduTypeReject || responses[0].RejectReason != types.RejectReasonParameterOutOfRange {
		t.Fatalf("priority 17 response = %+v, %v", responses, err)
	}

	property := device.Objects[object].Properties[uint32(types.PropertyPresentValue)]
	property.Scalar = true
	property.ExpectedTag = types.TagReal
	defaultValue := property.Values[0]
	property.RelinquishDefault = &defaultValue
	request.Apdu.Payload = append(append([]byte(nil), payload...), 0x49, 6)
	responses, err = application.handleWriteProperty(context.Background(), &responder.Request{Packet: request})
	if err != nil || len(responses) != 1 || responses[0].PDUType != types.PduTypeError || responses[0].ErrorCode != types.ErrorCodeWriteAccessDenied {
		t.Fatalf("reserved priority response = %+v, %v", responses, err)
	}

	wrongPayload, err := encodeReadPropertyResult(object, PropertyReference{ID: uint32(types.PropertyPresentValue)}, []Value{{Tag: types.TagCharacterString, Value: "wrong"}})
	if err != nil {
		t.Fatal(err)
	}
	request.Apdu.Payload = append(wrongPayload, 0x49, 8)
	responses, err = application.handleWriteProperty(context.Background(), &responder.Request{Packet: request})
	if err != nil || len(responses) != 1 || responses[0].PDUType != types.PduTypeError || responses[0].ErrorCode != types.ErrorCodeInvalidDataType {
		t.Fatalf("wrong data type response = %+v, %v", responses, err)
	}
}

func TestApplicationPrunesExpiredSubscriberEndpoints(t *testing.T) {
	clock := NewManualClock(time.Unix(1000, 0))
	application := NewApplication(applicationTestDevice(), clock)
	key := SubscriptionKey{Subscriber: "192.0.2.1:47808", ProcessID: 1, Object: ObjectID{Type: uint16(types.ObjectTypeAnalogValue), Instance: 1}}
	application.Subscriptions.Subscribe(Subscription{Key: key, Lifetime: time.Second})
	application.subscribers[key] = transport.NewEndpoint(net.IPv4(192, 0, 2, 1), 47808)
	clock.Advance(2 * time.Second)
	if active := application.activeSubscriptions(); len(active) != 0 {
		t.Fatalf("active subscriptions = %d", len(active))
	}
	if len(application.subscribers) != 0 {
		t.Fatalf("subscriber endpoints = %d", len(application.subscribers))
	}
}

func applicationTestDevice() *Device {
	objectID := ObjectID{Type: uint16(types.ObjectTypeAnalogValue), Instance: 1}
	return &Device{
		ID:       70000,
		Name:     "Test Device",
		VendorID: 260,
		Objects: map[ObjectID]*Object{
			objectID: {
				ID:   objectID,
				Name: "Setpoint",
				Properties: map[uint32]*Property{
					uint32(types.PropertyPresentValue): {ID: uint32(types.PropertyPresentValue), Writable: true, Scalar: true, ExpectedTag: types.TagReal, Values: []Value{{Tag: types.TagReal, Value: float32(20.5)}}},
					uint32(types.PropertyObjectName):   {ID: uint32(types.PropertyObjectName), Values: []Value{{Tag: types.TagCharacterString, Value: "Setpoint"}}},
				},
			},
		},
	}
}

func applicationPacket(t *testing.T, pduType types.PduType, service, invoke uint8, payload []byte, broadcast bool) []byte {
	t.Helper()
	packet := bacnet.NewRequest()
	defer packet.Release()
	packet.Header.Function = types.BvlcFunctionOriginalUnicastNpdu
	if broadcast {
		packet.Header.Function = types.BvlcFunctionOriginalBroadcastNpdu
	}
	packet.Npci.ExpectingReply = pduType == types.PduTypeConfirmedServiceRequest
	packet.Apdu.PduType = pduType
	packet.Apdu.ServiceChoice = service
	packet.Apdu.InvokeID = invoke
	packet.Apdu.Payload = payload
	encoded, err := packet.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func readApplicationResponse(t *testing.T, conn transport.Conn) *bacnet.Request {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	datagram, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := bacnet.ParseRequest(datagram.Payload, datagram.Source.UDPAddr())
	if err != nil {
		t.Fatal(err)
	}
	return packet
}
