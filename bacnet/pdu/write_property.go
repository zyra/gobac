package pdu

import (
	"github.com/zyra/gobac/bacnet/types"
)

type WriteProperty struct {
	Property *types.Property
	Priority uint8
}

func (p *WriteProperty) MarshalBinary() (b []byte, err error) {
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
