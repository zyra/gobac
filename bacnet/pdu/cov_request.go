package pdu

import (
	"bytes"
	"github.com/zyra/gobac/bacnet/types"
)

type SubscribeCov struct {
	ProcessIdentifier uint8
	ObjectId          *types.ObjectId
	Cancel            bool
	Timeout           uint32
}

func (p *SubscribeCov) MarshalBinary() ([]byte, error) {
	buff := buffPool.Get().(*bytes.Buffer)
	defer buff.Reset()
	defer buffPool.Put(buff)

	t := &types.Tag{}

	// Write process id tag & value
	t.TagNumber = 0
	t.LenValue = types.GetUintLen(uint(p.ProcessIdentifier))

	if _, err := buff.Write(t.EncodeContextTag()); err != nil {
		return nil, err
	}

	if err := buff.WriteByte(p.ProcessIdentifier); err != nil {
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
		t.TagNumber = 3
		t.LenValue = 4

		if _, err := buff.Write(t.EncodeContextTag()); err != nil {
			return nil, err
		}

		if _, err := buff.Write(types.EncodeVarUint(p.Timeout)); err != nil {
			return nil, err
		}
	}

	return buff.Bytes(), nil
}
