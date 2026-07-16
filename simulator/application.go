package simulator

import (
	"context"
	"math"
	"net"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/zyra/gobac/bacnet/responder"
	"github.com/zyra/gobac/bacnet/transport"
	"github.com/zyra/gobac/bacnet/types"
)

// Application exposes one simulated device through BACnet services.
type Application struct {
	Device        *Device
	Subscriptions *SubscriptionRegistry

	mu          sync.Mutex
	subscribers map[SubscriptionKey]transport.Endpoint
	invokeID    uint8
}

func NewApplication(device *Device, clock Clock) *Application {
	return &Application{
		Device:        device,
		Subscriptions: NewSubscriptionRegistry(clock),
		subscribers:   make(map[SubscriptionKey]transport.Endpoint),
	}
}

func (a *Application) Register(server *responder.Server) {
	server.Handle(types.PduTypeUnconfirmedServiceRequest, types.UnconfirmedServiceWhoIs, a.handleWhoIs)
	server.Handle(types.PduTypeConfirmedServiceRequest, types.ConfirmedServiceReadProperty, a.handleReadProperty)
	server.Handle(types.PduTypeConfirmedServiceRequest, types.ConfirmedServiceWriteProperty, a.handleWriteProperty)
	server.Handle(types.PduTypeConfirmedServiceRequest, types.ConfirmedServiceReadPropertyMultiple, a.handleReadPropertyMultiple)
	server.Handle(types.PduTypeConfirmedServiceRequest, types.ConfirmedServiceSubscribeCov, a.handleSubscribeCOV)
}

func (a *Application) handleWhoIs(_ context.Context, request *responder.Request) ([]responder.Response, error) {
	low, high, err := decodeWhoIs(request.Packet.Apdu.Payload)
	if err != nil {
		return nil, nil
	}
	if low != nil && (a.Device.ID < *low || a.Device.ID > *high) {
		return nil, nil
	}
	payload, err := encodeIAm(a.Device)
	if err != nil {
		return nil, err
	}
	destination := transport.NewEndpoint(net.IPv4bcast, request.Source.Port)
	return []responder.Response{
		responder.Unconfirmed(types.UnconfirmedServiceIAm, payload).To(destination).AsBroadcast(),
	}, nil
}

func (a *Application) handleReadProperty(_ context.Context, request *responder.Request) ([]responder.Response, error) {
	object, reference, err := decodeReadProperty(request.Packet.Apdu.Payload)
	if err != nil {
		return []responder.Response{responder.Reject(types.RejectReasonInvalidTag)}, nil
	}
	values, err := a.Device.ReadProperty(object, reference.ID, reference.ArrayIndex)
	if err != nil {
		return []responder.Response{modelError(err)}, nil
	}
	payload, err := encodeReadPropertyResult(object, reference, values)
	if err != nil {
		return []responder.Response{responder.ErrorResponse(types.ErrorClassProperty, types.ErrorCodeDatatypeNotSupported)}, nil
	}
	return []responder.Response{responder.ComplexACK(payload)}, nil
}

func (a *Application) handleWriteProperty(_ context.Context, request *responder.Request) ([]responder.Response, error) {
	object, reference, values, priority, err := decodeWriteProperty(request.Packet.Apdu.Payload)
	if err != nil {
		return []responder.Response{responder.Reject(types.RejectReasonInvalidTag)}, nil
	}
	if reference.ArrayIndex != nil {
		return []responder.Response{responder.ErrorResponse(types.ErrorClassProperty, types.ErrorCodeInvalidArrayIndex)}, nil
	}
	if err := a.Device.WriteProperty(object, reference.ID, values, priority); err != nil {
		return []responder.Response{modelError(err)}, nil
	}
	responses := []responder.Response{responder.SimpleACK()}
	responses = append(responses, a.notifications(object)...)
	return responses, nil
}

func (a *Application) handleReadPropertyMultiple(_ context.Context, request *responder.Request) ([]responder.Response, error) {
	specifications, err := decodeReadPropertyMultiple(request.Packet.Apdu.Payload)
	if err != nil {
		return []responder.Response{responder.Reject(types.RejectReasonInvalidTag)}, nil
	}
	results := make([]ReadAccessResult, 0, len(specifications))
	for _, specification := range specifications {
		access := ReadAccessResult{Object: specification.Object}
		for _, reference := range expandPropertyReferences(a.Device, specification) {
			propertyResult := PropertyResult{Reference: reference}
			propertyResult.Values, err = a.Device.ReadProperty(specification.Object, reference.ID, reference.ArrayIndex)
			if err != nil {
				response := modelError(err)
				propertyResult.ErrorClass = response.ErrorClass
				propertyResult.ErrorCode = response.ErrorCode
			}
			access.Results = append(access.Results, propertyResult)
		}
		results = append(results, access)
	}
	payload, err := encodeReadPropertyMultipleResult(results)
	if err != nil {
		return nil, err
	}
	return []responder.Response{responder.ComplexACK(payload)}, nil
}

func (a *Application) handleSubscribeCOV(_ context.Context, request *responder.Request) ([]responder.Response, error) {
	subscription, err := decodeSubscribeCOV(request.Packet.Apdu.Payload)
	if err != nil {
		return []responder.Response{responder.Reject(types.RejectReasonInvalidTag)}, nil
	}
	if _, err := a.Device.Object(subscription.Object); err != nil {
		return []responder.Response{modelError(err)}, nil
	}
	values, err := a.Device.ReadProperty(subscription.Object, uint32(types.PropertyPresentValue), nil)
	if err == ErrUnknownProperty {
		return []responder.Response{responder.ErrorResponse(types.ErrorClassProperty, types.ErrorCodeNotCovProperty)}, nil
	}
	if err != nil {
		return []responder.Response{modelError(err)}, nil
	}
	key := SubscriptionKey{
		Subscriber: request.Source.String(),
		ProcessID:  subscription.ProcessIdentifier,
		Object:     subscription.Object,
	}
	if subscription.Cancel {
		a.Subscriptions.Cancel(key)
		a.mu.Lock()
		delete(a.subscribers, key)
		a.mu.Unlock()
		return []responder.Response{responder.SimpleACK()}, nil
	}
	a.Subscriptions.Subscribe(Subscription{
		Key:       key,
		Confirmed: subscription.Confirmed,
		Lifetime:  time.Duration(subscription.Lifetime) * time.Second,
		LastValue: values,
	})
	a.mu.Lock()
	a.subscribers[key] = request.Source
	a.mu.Unlock()
	active := a.subscription(key)
	notification, err := a.notification(active, request.Source, values)
	if err != nil {
		return nil, err
	}
	return []responder.Response{responder.SimpleACK(), notification}, nil
}

func (a *Application) notifications(object ObjectID) []responder.Response {
	active := a.activeSubscriptions()
	responses := make([]responder.Response, 0, len(active))
	for _, subscription := range active {
		if subscription.Key.Object != object {
			continue
		}
		values, err := a.Device.ReadProperty(object, uint32(types.PropertyPresentValue), nil)
		if err != nil {
			continue
		}
		a.mu.Lock()
		endpoint, exists := a.subscribers[subscription.Key]
		a.mu.Unlock()
		if !exists {
			continue
		}
		if !a.covChanged(object, subscription.LastValue, values) {
			continue
		}
		notification, err := a.notification(subscription, endpoint, values)
		if err != nil {
			continue
		}
		a.Subscriptions.UpdateLastValue(subscription.Key, values)
		responses = append(responses, notification)
	}
	return responses
}

func (a *Application) covChanged(objectID ObjectID, previous, current []Value) bool {
	if len(previous) != len(current) {
		return true
	}
	object, err := a.Device.Object(objectID)
	if err != nil {
		return false
	}
	increment := float64(0)
	if property := object.Properties[uint32(types.PropertyPresentValue)]; property != nil {
		increment = property.COVIncrement
	}
	for i := range current {
		if increment > 0 {
			oldValue, oldErr := toPropertyValue(previous[i])
			newValue, newErr := toPropertyValue(current[i])
			if oldErr == nil && newErr == nil && oldValue.IsNumeric() && newValue.IsNumeric() {
				if math.Abs(newValue.ReadAsFloat64()-oldValue.ReadAsFloat64()) >= increment {
					return true
				}
				continue
			}
		}
		if previous[i].Tag != current[i].Tag || !reflect.DeepEqual(previous[i].Value, current[i].Value) {
			return true
		}
	}
	return false
}

func (a *Application) notification(subscription Subscription, endpoint transport.Endpoint, values []Value) (responder.Response, error) {
	timeRemaining := uint32(0)
	if !subscription.ExpiresAt.IsZero() {
		remaining := subscription.ExpiresAt.Sub(a.Subscriptions.clock.Now())
		if remaining > 0 {
			timeRemaining = uint32((remaining + time.Second - 1) / time.Second)
		}
	}
	payload, err := encodeCOVNotification(
		subscription.Key.ProcessID,
		a.Device.ID,
		subscription.Key.Object,
		timeRemaining,
		[]PropertyResult{{
			Reference: PropertyReference{ID: uint32(types.PropertyPresentValue)},
			Values:    values,
		}},
	)
	if err != nil {
		return responder.Response{}, err
	}
	if subscription.Confirmed {
		return responder.Confirmed(types.ConfirmedServiceCovNotification, a.nextInvokeID(), payload).To(endpoint), nil
	}
	return responder.Unconfirmed(types.UnconfirmedServiceCovNotification, payload).To(endpoint), nil
}

func (a *Application) subscription(key SubscriptionKey) Subscription {
	for _, subscription := range a.activeSubscriptions() {
		if subscription.Key == key {
			return subscription
		}
	}
	return Subscription{Key: key}
}

func (a *Application) activeSubscriptions() []Subscription {
	active := a.Subscriptions.Active()
	keys := make(map[SubscriptionKey]struct{}, len(active))
	for _, subscription := range active {
		keys[subscription.Key] = struct{}{}
	}
	a.mu.Lock()
	for key := range a.subscribers {
		if _, exists := keys[key]; !exists {
			delete(a.subscribers, key)
		}
	}
	a.mu.Unlock()
	return active
}

func (a *Application) nextInvokeID() uint8 {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.invokeID++
	if a.invokeID == 0 {
		a.invokeID = 1
	}
	return a.invokeID
}

func expandPropertyReferences(device *Device, specification ReadAccessSpecification) []PropertyReference {
	if len(specification.Properties) != 1 || specification.Properties[0].ID != uint32(types.PropertyAll) {
		return specification.Properties
	}
	object, err := device.Object(specification.Object)
	if err != nil {
		return specification.Properties
	}
	ids := make([]int, 0, len(object.Properties))
	for id := range object.Properties {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	result := make([]PropertyReference, 0, len(ids))
	for _, id := range ids {
		result = append(result, PropertyReference{ID: uint32(id)})
	}
	return result
}

func modelError(err error) responder.Response {
	switch err {
	case ErrUnknownObject:
		return responder.ErrorResponse(types.ErrorClassObject, types.ErrorCodeUnknownObject)
	case ErrUnknownProperty:
		return responder.ErrorResponse(types.ErrorClassProperty, types.ErrorCodeUnknownProperty)
	case ErrPropertyNotArray:
		return responder.ErrorResponse(types.ErrorClassProperty, types.ErrorCodePropertyIsNotAnArray)
	case ErrInvalidArrayIndex:
		return responder.ErrorResponse(types.ErrorClassProperty, types.ErrorCodeInvalidArrayIndex)
	case ErrWriteDenied:
		return responder.ErrorResponse(types.ErrorClassProperty, types.ErrorCodeWriteAccessDenied)
	case ErrInvalidPriority:
		return responder.ErrorResponse(types.ErrorClassProperty, types.ErrorCodeValueOutOfRange)
	default:
		return responder.ErrorResponse(types.ErrorClassServices, types.ErrorCodeServiceRequestDenied)
	}
}
