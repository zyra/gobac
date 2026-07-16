package pdu

import (
	"errors"

	"github.com/zyra/gobac/v2/bacnet/types"
)

type WriteProperty struct {
	Property *types.Property
	Priority uint8
}

func (p *WriteProperty) MarshalBinary() (b []byte, err error) {
	if p.Property == nil {
		return nil, errors.New("property is required")
	}
	if p.Priority > 16 {
		return nil, errors.New("write priority must be between 1 and 16")
	}
	propBytes, err := p.Property.MarshalBinary()

	if err != nil {
		return nil, err
	}

	if p.Priority > 0 {
		tag := &types.Tag{
			TagNumber: 4,
			LenValue:  1,
		}

		tagBytes := tag.EncodeContextTag()

		return append(propBytes, append(tagBytes, uint8(p.Priority))...), nil
	}

	return propBytes, nil
}
