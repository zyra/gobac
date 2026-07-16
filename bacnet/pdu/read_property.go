package pdu

import (
	"errors"

	"github.com/zyra/gobac/v2/bacnet/types"
)

type ReadPropertyPdu struct {
	Property     *types.Property
	RequireValue bool
}

func (p *ReadPropertyPdu) MarshalBinary() ([]byte, error) {
	return p.Property.MarshalBinary()
}

func (p *ReadPropertyPdu) GetPduType() uint8 {
	return uint8(types.PduTypeConfirmedServiceRequest)
}

func (p *ReadPropertyPdu) UnmarshalBinary(data []byte) (e error) {
	p.Property = &types.Property{}
	if err := p.Property.UnmarshalBinary(data); err != nil {
		return err
	}
	if p.RequireValue && len(p.Property.Values) == 0 {
		return errors.New("ReadProperty acknowledgement has no value")
	}
	return nil
}
