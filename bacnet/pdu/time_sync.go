package pdu

import (
	"bytes"
	"errors"

	"github.com/zyra/gobac/v2/bacnet/types"
)

// TimeSyncPdu is the TimeSynchronization-Request / UTCTimeSynchronization-Request
// payload (ASHRAE 135 §16.10/§16.11): a Date followed by a Time, both
// application-tagged. Neither service expects a reply.
type TimeSyncPdu struct {
	Date types.Date
	Time types.Time
}

func (p *TimeSyncPdu) MarshalBinary() ([]byte, error) {
	buff := bytes.NewBuffer(nil)

	dateVal := types.PropertyValue{Type: types.TagDate, Value: p.Date}
	encoded, err := dateVal.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buff.Write(encoded)

	timeVal := types.PropertyValue{Type: types.TagTime, Value: p.Time}
	encoded, err = timeVal.MarshalBinary()
	if err != nil {
		return nil, err
	}
	buff.Write(encoded)

	return buff.Bytes(), nil
}

func (p *TimeSyncPdu) UnmarshalBinary(b []byte) error {
	values, err := decodePropertyResultValues(b)
	if err != nil {
		return err
	}
	if len(values) != 2 {
		return errors.New("time synchronization expects exactly two application-tagged values")
	}

	dateVal, ok := values[0].Value.(types.Date)
	if !ok || values[0].Type != types.TagDate {
		return errors.New("expected a date")
	}
	timeVal, ok := values[1].Value.(types.Time)
	if !ok || values[1].Type != types.TagTime {
		return errors.New("expected a time")
	}

	p.Date = dateVal
	p.Time = timeVal
	return nil
}
