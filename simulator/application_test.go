package simulator

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/zyra/gobac/v2/bacnet"
	"github.com/zyra/gobac/v2/bacnet/pdu"
	"github.com/zyra/gobac/v2/bacnet/responder"
	"github.com/zyra/gobac/v2/bacnet/transport"
	"github.com/zyra/gobac/v2/bacnet/types"
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

func whoHasTestRequest(t *testing.T, query *pdu.WhoHas) *responder.Request {
	t.Helper()
	payload, err := query.MarshalBinary()
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

func TestApplicationHandleWhoHasByObjectId(t *testing.T) {
	device := applicationTestDevice()
	application := NewApplication(device, RealClock{})

	request := whoHasTestRequest(t, &pdu.WhoHas{
		ObjectId: &types.ObjectId{Type: types.ObjectTypeAnalogValue, Instance: 1},
	})
	responses, err := application.handleWhoHas(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(responses) != 1 {
		t.Fatalf("responses = %+v, want exactly one", responses)
	}
	response := responses[0]
	if response.PDUType != types.PduTypeUnconfirmedServiceRequest || response.ServiceChoice != types.UnconfirmedServiceIHave {
		t.Fatalf("unexpected I-Have response: %+v", response)
	}
	iHave := &pdu.IHave{}
	if err := iHave.UnmarshalBinary(response.Payload); err != nil {
		t.Fatal(err)
	}
	wantDeviceId := types.ObjectId{Type: types.ObjectTypeDevice}
	if err := wantDeviceId.SetInstanceNumber(device.ID); err != nil {
		t.Fatal(err)
	}
	wantObjectId := types.ObjectId{Type: types.ObjectTypeAnalogValue, Instance: 1}
	if iHave.DeviceId != wantDeviceId || iHave.ObjectId != wantObjectId || iHave.ObjectName != "Setpoint" {
		t.Fatalf("i-have = %+v, want device=%+v object=%+v name=%q", iHave, wantDeviceId, wantObjectId, "Setpoint")
	}
}

func TestApplicationHandleWhoHasByNameMissing(t *testing.T) {
	device := applicationTestDevice()
	application := NewApplication(device, RealClock{})

	request := whoHasTestRequest(t, &pdu.WhoHas{ObjectName: "does-not-exist"})
	responses, err := application.handleWhoHas(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(responses) != 0 {
		t.Fatalf("responses = %+v, want none", responses)
	}
}

func TestApplicationHandleWhoHasRangeExcludesDevice(t *testing.T) {
	device := applicationTestDevice() // device.ID == 70000
	application := NewApplication(device, RealClock{})

	request := whoHasTestRequest(t, &pdu.WhoHas{
		HasRange:  true,
		LowLimit:  1,
		HighLimit: 100,
		ObjectId:  &types.ObjectId{Type: types.ObjectTypeAnalogValue, Instance: 1},
	})
	responses, err := application.handleWhoHas(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(responses) != 0 {
		t.Fatalf("responses = %+v, want none", responses)
	}
}

// writePropertyMultipleTestPayload is the WritePropertyMultiple-Request wire
// encoding for two objects: analog-value 1 present-value <- Real 21.0
// priority 8, and binary-value 2 present-value <- Enumerated 1, no priority.
var writePropertyMultipleTestPayload = []byte{
	0x0c, 0x00, 0x80, 0x00, 0x01, // [0] objectIdentifier AV:1
	0x1e,       // [1] opening
	0x09, 0x55, //   [0] propertyIdentifier 85
	0x2e,                         //   [2] opening
	0x44, 0x41, 0xa8, 0x00, 0x00, //     Real 21.0
	0x2f,       //   [2] closing
	0x39, 0x08, //   [3] priority 8
	0x1f, // [1] closing

	0x0c, 0x01, 0x40, 0x00, 0x02, // [0] objectIdentifier BV:2
	0x1e,       // [1] opening
	0x09, 0x55, //   [0] propertyIdentifier 85
	0x2e,       //   [2] opening
	0x91, 0x01, //     Enumerated 1
	0x2f, //   [2] closing
	0x1f, // [1] closing (no priority)
}

func writePropertyMultipleTestDevice(binaryWritable bool) (*Device, ObjectID, ObjectID) {
	analogID := ObjectID{Type: uint16(types.ObjectTypeAnalogValue), Instance: 1}
	binaryID := ObjectID{Type: uint16(types.ObjectTypeBinaryValue), Instance: 2}
	defaultValue := Value{Tag: types.TagReal, Value: float32(20.5)}
	device := &Device{
		ID:       70000,
		Name:     "Test Device",
		VendorID: 260,
		Objects: map[ObjectID]*Object{
			analogID: {
				ID:   analogID,
				Name: "Setpoint",
				Properties: map[uint32]*Property{
					// Commandable (has a relinquish default) so the priority-8
					// write in writePropertyMultipleTestPayload is valid.
					uint32(types.PropertyPresentValue): {
						ID:                uint32(types.PropertyPresentValue),
						Writable:          true,
						Scalar:            true,
						ExpectedTag:       types.TagReal,
						RelinquishDefault: &defaultValue,
					},
				},
			},
			binaryID: {
				ID:   binaryID,
				Name: "Status",
				Properties: map[uint32]*Property{
					uint32(types.PropertyPresentValue): {
						ID:          uint32(types.PropertyPresentValue),
						Writable:    binaryWritable,
						Scalar:      true,
						ExpectedTag: types.TagEnumerated,
						Values:      []Value{{Tag: types.TagEnumerated, Value: uint32(0)}},
					},
				},
			},
		},
	}
	return device, analogID, binaryID
}

func TestApplicationWritePropertyMultipleAllValid(t *testing.T) {
	device, analogID, binaryID := writePropertyMultipleTestDevice(true)
	application := NewApplication(device, RealClock{})

	subscriber := transport.NewEndpoint(net.IPv4(192, 0, 2, 1), 47808)
	key := SubscriptionKey{Subscriber: subscriber.String(), ProcessID: 1, Object: analogID}
	application.Subscriptions.Subscribe(Subscription{
		Key:       key,
		Lifetime:  time.Hour,
		LastValue: []Value{{Tag: types.TagReal, Value: float32(20.5)}},
	})
	application.subscribers[key] = subscriber

	request := bacnet.NewRequest()
	defer request.Release()
	request.Apdu.Payload = writePropertyMultipleTestPayload
	responses, err := application.handleWritePropertyMultiple(context.Background(), &responder.Request{Packet: request})
	if err != nil {
		t.Fatal(err)
	}
	if len(responses) != 2 || responses[0].PDUType != types.PduTypeSimpleAck {
		t.Fatalf("responses = %+v, err = %v", responses, err)
	}
	notification := responses[1]
	if notification.PDUType != types.PduTypeUnconfirmedServiceRequest || notification.ServiceChoice != types.UnconfirmedServiceCovNotification {
		t.Fatalf("notification response = %+v", notification)
	}

	analogValues, err := device.ReadProperty(analogID, uint32(types.PropertyPresentValue), nil)
	if err != nil || len(analogValues) != 1 || analogValues[0].Value != types.Real(21.0) {
		t.Fatalf("analog present-value = %+v, err = %v", analogValues, err)
	}
	binaryValues, err := device.ReadProperty(binaryID, uint32(types.PropertyPresentValue), nil)
	if err != nil || len(binaryValues) != 1 || binaryValues[0].Value != uint32(1) {
		t.Fatalf("binary present-value = %+v, err = %v", binaryValues, err)
	}
}

func TestApplicationWritePropertyMultipleSecondWriteInvalidIsAtomic(t *testing.T) {
	device, analogID, binaryID := writePropertyMultipleTestDevice(false)
	application := NewApplication(device, RealClock{})

	request := bacnet.NewRequest()
	defer request.Release()
	request.Apdu.Payload = writePropertyMultipleTestPayload
	responses, err := application.handleWritePropertyMultiple(context.Background(), &responder.Request{Packet: request})
	if err != nil {
		t.Fatal(err)
	}
	if len(responses) != 1 || responses[0].PDUType != types.PduTypeError {
		t.Fatalf("responses = %+v, err = %v", responses, err)
	}
	if responses[0].ErrorClass != types.ErrorClassProperty || responses[0].ErrorCode != types.ErrorCodeWriteAccessDenied {
		t.Fatalf("error class/code = %v/%v", responses[0].ErrorClass, responses[0].ErrorCode)
	}

	wpmError := &pdu.WritePropertyMultipleError{}
	if err := wpmError.UnmarshalBinary(responses[0].Payload); err != nil {
		t.Fatal(err)
	}
	if wpmError.Class != types.ErrorClassProperty || wpmError.Code != types.ErrorCodeWriteAccessDenied {
		t.Fatalf("decoded error class/code = %v/%v", wpmError.Class, wpmError.Code)
	}
	wantObject := ObjectID{Type: uint16(wpmError.FirstFailed.ObjectId.Type), Instance: wpmError.FirstFailed.ObjectId.InstanceNumber()}
	if wantObject != binaryID || wpmError.FirstFailed.ID != uint32(types.PropertyPresentValue) {
		t.Fatalf("firstFailed = object %+v property %d, want object %+v property %d", wantObject, wpmError.FirstFailed.ID, binaryID, uint32(types.PropertyPresentValue))
	}

	// Atomicity: the first object's write must not have been applied.
	analogValues, err := device.ReadProperty(analogID, uint32(types.PropertyPresentValue), nil)
	if err != nil || len(analogValues) != 1 || analogValues[0].Value != float32(20.5) {
		t.Fatalf("analog present-value = %+v, err = %v, want unchanged 20.5", analogValues, err)
	}
}

func TestApplicationWritePropertyMultipleReservedPriority(t *testing.T) {
	device, analogID, _ := writePropertyMultipleTestDevice(true)
	application := NewApplication(device, RealClock{})

	payload := []byte{
		0x0c, 0x00, 0x80, 0x00, 0x01, // [0] objectIdentifier AV:1
		0x1e,       // [1] opening
		0x09, 0x55, //   [0] propertyIdentifier 85
		0x2e,                         //   [2] opening
		0x44, 0x41, 0xa8, 0x00, 0x00, //     Real 21.0
		0x2f,       //   [2] closing
		0x39, 0x06, //   [3] priority 6 (reserved)
		0x1f, // [1] closing
	}

	request := bacnet.NewRequest()
	defer request.Release()
	request.Apdu.Payload = payload
	responses, err := application.handleWritePropertyMultiple(context.Background(), &responder.Request{Packet: request})
	if err != nil {
		t.Fatal(err)
	}
	if len(responses) != 1 || responses[0].PDUType != types.PduTypeError {
		t.Fatalf("responses = %+v, err = %v", responses, err)
	}
	wantClass, wantCode := modelErrorClassCode(ErrReservedPriority)
	if responses[0].ErrorClass != wantClass || responses[0].ErrorCode != wantCode {
		t.Fatalf("error class/code = %v/%v, want %v/%v", responses[0].ErrorClass, responses[0].ErrorCode, wantClass, wantCode)
	}

	// Nothing was mutated: the command priority array remains empty and the
	// property still reports its relinquish-default value.
	values, err := device.ReadProperty(analogID, uint32(types.PropertyPresentValue), nil)
	if err != nil || len(values) != 1 || values[0].Value != float32(20.5) {
		t.Fatalf("present-value = %+v, err = %v, want unchanged relinquish-default 20.5", values, err)
	}
}

func TestReadPropertyPriorityArrayOverWire(t *testing.T) {
	objectID := ObjectID{Type: uint16(types.ObjectTypeAnalogValue), Instance: 1}
	defaultValue := Value{Tag: types.TagReal, Value: float32(20.5)}
	device := &Device{
		ID:       70001,
		Name:     "Test Device",
		VendorID: 260,
		Objects: map[ObjectID]*Object{
			objectID: {
				ID:   objectID,
				Name: "Setpoint",
				Properties: map[uint32]*Property{
					uint32(types.PropertyPresentValue): {
						ID:                uint32(types.PropertyPresentValue),
						Writable:          true,
						Scalar:            true,
						ExpectedTag:       types.TagReal,
						RelinquishDefault: &defaultValue,
					},
				},
			},
		},
	}
	if err := device.WriteProperty(objectID, uint32(types.PropertyPresentValue), []Value{{Tag: types.TagReal, Value: float32(21)}}, 8); err != nil {
		t.Fatal(err)
	}

	application := NewApplication(device, RealClock{})
	bacnetObjectID, err := toBACnetObjectID(objectID)
	if err != nil {
		t.Fatal(err)
	}

	requestPayload, err := (&types.Property{ObjectId: bacnetObjectID, ID: types.PropertyPriorityArray}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	request := bacnet.NewRequest()
	defer request.Release()
	request.Apdu.Payload = requestPayload
	responses, err := application.handleReadProperty(context.Background(), &responder.Request{Packet: request})
	if err != nil || len(responses) != 1 || responses[0].PDUType != types.PduTypeComplexAck {
		t.Fatalf("full-array responses = %+v, %v", responses, err)
	}

	result := &types.Property{}
	if err := result.UnmarshalBinary(responses[0].Payload); err != nil {
		t.Fatal(err)
	}
	if len(result.Values) != PrioritySlots {
		t.Fatalf("values = %d, want %d", len(result.Values), PrioritySlots)
	}
	for i, value := range result.Values {
		if i == 7 { // priority 8 (1-indexed) is slice index 7.
			if value.Type != types.TagReal || value.ReadAsFloat64() != 21 {
				t.Fatalf("slot 8 = %+v, want 21", value)
			}
			continue
		}
		if value.Type != types.TagNull {
			t.Fatalf("slot %d = %+v, want NULL", i+1, value)
		}
	}

	indexedPayload, err := (&types.Property{ObjectId: bacnetObjectID, ID: types.PropertyPriorityArray, HasIndex: true, Index: 16}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	request.Apdu.Payload = indexedPayload
	responses, err = application.handleReadProperty(context.Background(), &responder.Request{Packet: request})
	if err != nil || len(responses) != 1 || responses[0].PDUType != types.PduTypeComplexAck {
		t.Fatalf("indexed responses = %+v, %v", responses, err)
	}
	indexed := &types.Property{}
	if err := indexed.UnmarshalBinary(responses[0].Payload); err != nil {
		t.Fatal(err)
	}
	if len(indexed.Values) != 1 || indexed.Values[0].Type != types.TagNull {
		t.Fatalf("indexed value (unset slot 16) = %+v, want single NULL", indexed.Values)
	}
}

func TestExpandPropertyReferencesIncludesPriorityArrayForCommandableObjects(t *testing.T) {
	commandableObjectID := ObjectID{Type: uint16(types.ObjectTypeAnalogOutput), Instance: 1}
	defaultValue := Value{Tag: types.TagReal, Value: float32(0)}
	commandableDevice := &Device{
		ID: 1,
		Objects: map[ObjectID]*Object{
			commandableObjectID: {
				ID: commandableObjectID,
				Properties: map[uint32]*Property{
					uint32(types.PropertyPresentValue): {
						ID:                uint32(types.PropertyPresentValue),
						RelinquishDefault: &defaultValue,
						Values:            []Value{{Tag: types.TagReal, Value: float32(0)}},
					},
					uint32(types.PropertyObjectName): {
						ID:     uint32(types.PropertyObjectName),
						Values: []Value{{Tag: types.TagCharacterString, Value: "command"}},
					},
				},
			},
		},
	}
	spec := ReadAccessSpecification{Object: commandableObjectID, Properties: []PropertyReference{{ID: uint32(types.PropertyAll)}}}
	references := expandPropertyReferences(commandableDevice, spec)
	found := false
	for _, reference := range references {
		if reference.ID == uint32(types.PropertyPriorityArray) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expanded references = %+v, want PropertyPriorityArray included", references)
	}

	nonCommandableObjectID := ObjectID{Type: uint16(types.ObjectTypeAnalogInput), Instance: 2}
	nonCommandableDevice := &Device{
		ID: 2,
		Objects: map[ObjectID]*Object{
			nonCommandableObjectID: {
				ID: nonCommandableObjectID,
				Properties: map[uint32]*Property{
					uint32(types.PropertyPresentValue): {
						ID:     uint32(types.PropertyPresentValue),
						Values: []Value{{Tag: types.TagReal, Value: float32(10)}},
					},
				},
			},
		},
	}
	nonCommandableSpec := ReadAccessSpecification{Object: nonCommandableObjectID, Properties: []PropertyReference{{ID: uint32(types.PropertyAll)}}}
	nonCommandableReferences := expandPropertyReferences(nonCommandableDevice, nonCommandableSpec)
	for _, reference := range nonCommandableReferences {
		if reference.ID == uint32(types.PropertyPriorityArray) {
			t.Fatalf("non-commandable expansion incorrectly included PropertyPriorityArray: %+v", nonCommandableReferences)
		}
	}
}

func TestExpandPropertyReferencesAllIncludesRealismProperties(t *testing.T) {
	commandableObjectID := ObjectID{Type: uint16(types.ObjectTypeAnalogOutput), Instance: 1}
	defaultValue := Value{Tag: types.TagReal, Value: float32(0)}
	commandableDevice := &Device{
		ID: 1,
		Objects: map[ObjectID]*Object{
			commandableObjectID: {
				ID: commandableObjectID,
				Properties: map[uint32]*Property{
					uint32(types.PropertyPresentValue): {
						ID:                uint32(types.PropertyPresentValue),
						RelinquishDefault: &defaultValue,
						Values:            []Value{{Tag: types.TagReal, Value: float32(0)}},
					},
					uint32(types.PropertyObjectName): {
						ID:     uint32(types.PropertyObjectName),
						Values: []Value{{Tag: types.TagCharacterString, Value: "command"}},
					},
				},
			},
		},
	}
	spec := ReadAccessSpecification{Object: commandableObjectID, Properties: []PropertyReference{{ID: uint32(types.PropertyAll)}}}
	references := expandPropertyReferences(commandableDevice, spec)
	got := make(map[uint32]bool, len(references))
	for _, reference := range references {
		got[reference.ID] = true
	}
	for _, want := range []uint32{
		uint32(types.PropertyPresentValue),
		uint32(types.PropertyObjectName),
		uint32(types.PropertyStatusFlags),
		uint32(types.PropertyEventState),
		uint32(types.PropertyReliability),
		uint32(types.PropertyOutOfService),
		uint32(types.PropertyPriorityArray),
		uint32(types.PropertyRelinquishDefault),
	} {
		if !got[want] {
			t.Fatalf("expanded references = %+v, missing property %d", references, want)
		}
	}
	if len(got) != 8 {
		t.Fatalf("expanded references = %+v, want exactly 8 distinct property ids, got %d", references, len(got))
	}

	nonCommandableObjectID := ObjectID{Type: uint16(types.ObjectTypeAnalogInput), Instance: 2}
	nonCommandableDevice := &Device{
		ID: 2,
		Objects: map[ObjectID]*Object{
			nonCommandableObjectID: {
				ID: nonCommandableObjectID,
				Properties: map[uint32]*Property{
					uint32(types.PropertyPresentValue): {
						ID:     uint32(types.PropertyPresentValue),
						Values: []Value{{Tag: types.TagReal, Value: float32(10)}},
					},
				},
			},
		},
	}
	nonCommandableSpec := ReadAccessSpecification{Object: nonCommandableObjectID, Properties: []PropertyReference{{ID: uint32(types.PropertyAll)}}}
	nonCommandableReferences := expandPropertyReferences(nonCommandableDevice, nonCommandableSpec)
	nonCommandableGot := make(map[uint32]bool, len(nonCommandableReferences))
	for _, reference := range nonCommandableReferences {
		nonCommandableGot[reference.ID] = true
	}
	for _, want := range []uint32{
		uint32(types.PropertyPresentValue),
		uint32(types.PropertyStatusFlags),
		uint32(types.PropertyEventState),
		uint32(types.PropertyReliability),
		uint32(types.PropertyOutOfService),
	} {
		if !nonCommandableGot[want] {
			t.Fatalf("non-commandable expanded references = %+v, missing property %d", nonCommandableReferences, want)
		}
	}
	for _, absent := range []uint32{uint32(types.PropertyPriorityArray), uint32(types.PropertyRelinquishDefault)} {
		if nonCommandableGot[absent] {
			t.Fatalf("non-commandable expansion incorrectly included property %d: %+v", absent, nonCommandableReferences)
		}
	}
}

func TestOutOfServiceWriteDoesNotTriggerPresentValueCOVNotification(t *testing.T) {
	device := applicationTestDevice()
	objectID := ObjectID{Type: uint16(types.ObjectTypeAnalogValue), Instance: 1}
	application := NewApplication(device, RealClock{})

	subscriber := transport.NewEndpoint(net.IPv4(192, 0, 2, 1), 47808)
	presentValue, err := device.ReadProperty(objectID, uint32(types.PropertyPresentValue), nil)
	if err != nil {
		t.Fatal(err)
	}
	key := SubscriptionKey{Subscriber: subscriber.String(), ProcessID: 1, Object: objectID}
	application.Subscriptions.Subscribe(Subscription{
		Key:       key,
		Lifetime:  time.Hour,
		LastValue: presentValue,
	})
	application.subscribers[key] = subscriber

	outOfServicePayload, err := encodeReadPropertyResult(objectID, PropertyReference{ID: uint32(types.PropertyOutOfService)}, []Value{{Tag: types.TagBoolean, Value: true}})
	if err != nil {
		t.Fatal(err)
	}
	request := bacnet.NewRequest()
	defer request.Release()
	request.Apdu.Payload = outOfServicePayload
	responses, err := application.handleWriteProperty(context.Background(), &responder.Request{Packet: request})
	if err != nil {
		t.Fatal(err)
	}
	if len(responses) != 1 || responses[0].PDUType != types.PduTypeSimpleAck {
		t.Fatalf("Out_Of_Service write responses = %+v, err = %v, want a single SimpleACK and no COV notification", responses, err)
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

func TestHandleWhoIsRangeFiltering(t *testing.T) {
	application := NewApplication(applicationTestDevice(), RealClock{})
	destination := transport.NewEndpoint(net.IPv4(192, 0, 2, 20), 47808)

	buildPayload := func(low, high uint32) []byte {
		var payload bytes.Buffer
		writeContext(&payload, 0, types.EncodeVarUint(low))
		writeContext(&payload, 1, types.EncodeVarUint(high))
		return payload.Bytes()
	}

	invoke := func(payload []byte) ([]responder.Response, error) {
		request := bacnet.NewRequest()
		defer request.Release()
		request.Apdu.Payload = payload
		return application.handleWhoIs(context.Background(), &responder.Request{Packet: request, Destination: destination})
	}

	// Out-of-range Who-Is (device 70000 is above the queried range): no I-Am at all.
	responses, err := invoke(buildPayload(1, 100))
	if err != nil || len(responses) != 0 {
		t.Fatalf("out-of-range Who-Is responses = %+v, err = %v, want none", responses, err)
	}

	// Exact boundary match: low == high == device ID.
	responses, err = invoke(buildPayload(70000, 70000))
	if err != nil || len(responses) != 1 {
		t.Fatalf("boundary Who-Is responses = %+v, err = %v, want 1", responses, err)
	}
	if responses[0].PDUType != types.PduTypeUnconfirmedServiceRequest || responses[0].ServiceChoice != types.UnconfirmedServiceIAm || responses[0].Broadcast != true {
		t.Fatalf("boundary Who-Is response = %+v", responses[0])
	}

	// Range entirely above the device ID: no I-Am (covers the "> *high" side already; this covers "< *low").
	responses, err = invoke(buildPayload(70001, 80000))
	if err != nil || len(responses) != 0 {
		t.Fatalf("above-range Who-Is responses = %+v, err = %v, want none", responses, err)
	}

	// Empty (unlimited) Who-Is must still answer.
	responses, err = invoke(nil)
	if err != nil || len(responses) != 1 {
		t.Fatalf("unlimited Who-Is responses = %+v, err = %v, want 1", responses, err)
	}
}

func TestReadPropertyModelErrorMapping(t *testing.T) {
	device := applicationTestDevice()
	application := NewApplication(device, RealClock{})
	objectID := ObjectID{Type: uint16(types.ObjectTypeAnalogValue), Instance: 1}
	requestObjectID, err := toBACnetObjectID(objectID)
	if err != nil {
		t.Fatal(err)
	}
	unknownObjectID, err := toBACnetObjectID(ObjectID{Type: uint16(types.ObjectTypeAnalogValue), Instance: 99})
	if err != nil {
		t.Fatal(err)
	}

	// A commandable fixture (RelinquishDefault set) synthesizes Priority_Array (87) at read
	// time (Task 6); index 17 is out of range for the 16-slot array.
	commandableObjectID := ObjectID{Type: uint16(types.ObjectTypeAnalogOutput), Instance: 1}
	defaultValue := Value{Tag: types.TagReal, Value: float32(0)}
	commandableDevice := &Device{
		ID: 70004,
		Objects: map[ObjectID]*Object{
			commandableObjectID: {
				ID: commandableObjectID,
				Properties: map[uint32]*Property{
					uint32(types.PropertyPresentValue): {
						ID:                uint32(types.PropertyPresentValue),
						Writable:          true,
						Scalar:            true,
						ExpectedTag:       types.TagReal,
						RelinquishDefault: &defaultValue,
					},
				},
			},
		},
	}
	commandableApplication := NewApplication(commandableDevice, RealClock{})
	commandableObjectIDBACnet, err := toBACnetObjectID(commandableObjectID)
	if err != nil {
		t.Fatal(err)
	}

	unknownObjectPayload, err := (&types.Property{ObjectId: unknownObjectID, ID: types.PropertyPresentValue}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	unknownPropertyPayload, err := (&types.Property{ObjectId: requestObjectID, ID: types.PropertyId(999)}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	notArrayPayload, err := (&types.Property{ObjectId: requestObjectID, ID: types.PropertyPresentValue, HasIndex: true, Index: 1}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	invalidArrayIndexPayload, err := (&types.Property{ObjectId: commandableObjectIDBACnet, ID: types.PropertyPriorityArray, HasIndex: true, Index: 17}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		app       *Application
		payload   []byte
		wantClass types.ErrorClass
		wantCode  types.ErrorCode
	}{
		{"unknown object", application, unknownObjectPayload, types.ErrorClassObject, types.ErrorCodeUnknownObject},
		{"unknown property", application, unknownPropertyPayload, types.ErrorClassProperty, types.ErrorCodeUnknownProperty},
		{"property not array", application, notArrayPayload, types.ErrorClassProperty, types.ErrorCodePropertyIsNotAnArray},
		{"invalid array index", commandableApplication, invalidArrayIndexPayload, types.ErrorClassProperty, types.ErrorCodeInvalidArrayIndex},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			request := bacnet.NewRequest()
			defer request.Release()
			request.Apdu.Payload = tc.payload
			responses, err := tc.app.handleReadProperty(context.Background(), &responder.Request{Packet: request})
			if err != nil || len(responses) != 1 || responses[0].PDUType != types.PduTypeError {
				t.Fatalf("%s: responses = %+v, err = %v", tc.name, responses, err)
			}
			if responses[0].ErrorClass != tc.wantClass || responses[0].ErrorCode != tc.wantCode {
				t.Fatalf("%s: error = %+v, want class=%v code=%v", tc.name, responses[0], tc.wantClass, tc.wantCode)
			}
		})
	}
}

func TestWritePropertyValueOutOfRangeMapping(t *testing.T) {
	objectID := ObjectID{Type: uint16(types.ObjectTypeAnalogValue), Instance: 1}
	device := &Device{
		ID: 70005,
		Objects: map[ObjectID]*Object{
			objectID: {
				ID: objectID,
				Properties: map[uint32]*Property{
					uint32(types.PropertyPresentValue): {
						ID:              uint32(types.PropertyPresentValue),
						Scalar:          true,
						ExpectedTag:     types.TagUnsigned,
						MinimumUnsigned: 1,
						MaximumUnsigned: 3,
						Writable:        true,
					},
				},
			},
		},
	}
	application := NewApplication(device, RealClock{})

	payload, err := encodeReadPropertyResult(objectID, PropertyReference{ID: uint32(types.PropertyPresentValue)}, []Value{{Tag: types.TagUnsigned, Value: uint32(4)}})
	if err != nil {
		t.Fatal(err)
	}

	request := bacnet.NewRequest()
	defer request.Release()
	request.Apdu.Payload = payload
	responses, err := application.handleWriteProperty(context.Background(), &responder.Request{Packet: request})
	if err != nil || len(responses) != 1 || responses[0].PDUType != types.PduTypeError {
		t.Fatalf("responses = %+v, err = %v", responses, err)
	}
	if responses[0].ErrorClass != types.ErrorClassProperty || responses[0].ErrorCode != types.ErrorCodeValueOutOfRange {
		t.Fatalf("error = %+v, want class=%v code=%v", responses[0], types.ErrorClassProperty, types.ErrorCodeValueOutOfRange)
	}
}

func TestModelErrorDefaultBranch(t *testing.T) {
	defaultResponse := modelError(errors.New("unmapped"))
	if defaultResponse.PDUType != types.PduTypeError || defaultResponse.ErrorClass != types.ErrorClassServices || defaultResponse.ErrorCode != types.ErrorCodeServiceRequestDenied {
		t.Fatalf("default mapping = %+v", defaultResponse)
	}

	unknownObjectResponse := modelError(ErrUnknownObject)
	if unknownObjectResponse.PDUType != types.PduTypeError || unknownObjectResponse.ErrorClass != types.ErrorClassObject || unknownObjectResponse.ErrorCode != types.ErrorCodeUnknownObject {
		t.Fatalf("ErrUnknownObject mapping = %+v", unknownObjectResponse)
	}

	invalidArrayIndexResponse := modelError(ErrInvalidArrayIndex)
	if invalidArrayIndexResponse.PDUType != types.PduTypeError || invalidArrayIndexResponse.ErrorClass != types.ErrorClassProperty || invalidArrayIndexResponse.ErrorCode != types.ErrorCodeInvalidArrayIndex {
		t.Fatalf("ErrInvalidArrayIndex mapping = %+v", invalidArrayIndexResponse)
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
