package pdu

import (
	"bytes"
	"errors"

	"github.com/zyra/gobac/v2/bacnet/types"
)

type SubscribeCov struct {
	ProcessIdentifier   uint8
	ProcessIdentifier32 uint32
	ObjectId            *types.ObjectId
	Cancel              bool
	Timeout             uint32
}

func (p *SubscribeCov) MarshalBinary() ([]byte, error) {
	if p.ObjectId == nil {
		return nil, errors.New("COV subscription object identifier is required")
	}
	buff := buffPool.Get().(*bytes.Buffer)
	defer func() {
		buff.Reset()
		buffPool.Put(buff)
	}()

	t := &types.Tag{}

	// Write process id tag & value
	t.TagNumber = 0
	processID := p.ProcessIdentifier32
	if processID == 0 {
		processID = uint32(p.ProcessIdentifier)
	}
	processIdentifier := types.EncodeVarUint(processID)
	t.LenValue = len(processIdentifier)

	if _, err := buff.Write(t.EncodeContextTag()); err != nil {
		return nil, err
	}

	if _, err := buff.Write(processIdentifier); err != nil {
		return nil, err
	}

	// write obj id tag & value

	t.TagNumber = 1
	t.LenValue = 4

	if _, err := buff.Write(t.EncodeContextTag()); err != nil {
		return nil, err
	}

	if objIdBytes, err := p.ObjectId.MarshalBinary(); err != nil {
		return nil, err
	} else if _, err := buff.Write(objIdBytes); err != nil {
		return nil, err
	}

	if !p.Cancel {
		// ask for confirmed notifications
		// let's encode the context tag first
		t.TagNumber = 2
		t.LenValue = 1
		if _, err := buff.Write(t.EncodeContextTag()); err != nil {
			return nil, err
		}

		// 1 = confirmed COV
		// 2 = unconfirmed COV
		// it's hardcoded to confirmed for now
		if err := buff.WriteByte(1); err != nil {
			return nil, err
		}

		// Set timeout
		timeout := types.EncodeVarUint(p.Timeout)
		t.TagNumber = 3
		t.LenValue = len(timeout)

		if _, err := buff.Write(t.EncodeContextTag()); err != nil {
			return nil, err
		}

		if _, err := buff.Write(timeout); err != nil {
			return nil, err
		}
	}

	return buff.Bytes(), nil
}
