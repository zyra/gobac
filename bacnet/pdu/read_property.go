package pdu

import (
	"github.com/zyra/gobac/bacnet/types"
)

type ReadPropertyPdu struct {
	Property *types.Property
}

func (p *ReadPropertyPdu) MarshalBinary() ([]byte, error) {
	return p.Property.MarshalBinary()
}

func (p *ReadPropertyPdu) GetPduType() uint8 {
	return uint8(types.PduTypeConfirmedServiceRequest)
}

func (p *ReadPropertyPdu) UnmarshalBinary(data []byte) (e error) {
	p.Property = &types.Property{}
	return p.Property.UnmarshalBinary(data)
}
