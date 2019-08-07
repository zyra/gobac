package types

import (
	"fmt"
)

type Header struct {
	ProtocolType uint8
	Function     BvlcFunction
	BvlcLength   Uint16
	NsduLength   Uint16
}

func (h *Header) IsBroadcast() bool {
	return h.Function == BvlcFunctionOriginalBroadcastNpdu
}

func (h *Header) MarshalBinary() (b []byte, e error) {
	b = make([]byte, 4)
	b[0] = BACnetProtocol
	b[1] = byte(h.Function)

	if b2, err := h.BvlcLength.MarshalBinary(); err != nil {
		return nil, err
	} else {
		copy(b[2:], b2)
	}

	return b, nil
}

func (h *Header) UnmarshalBinary(b []byte) error {
	buff := GetBuff(b...)
	defer ReleaseBuff(buff)

	if b, e := buff.ReadByte(); e != nil {
		return e
	} else {
		h.ProtocolType = b
	}

	if h.ProtocolType != BACnetProtocol {
		return fmt.Errorf("expected protocol to be %x but got %x", BACnetProtocol, h.ProtocolType)
	}

	if b, e := buff.ReadByte(); e != nil {
		return e
	} else {
		h.Function = BvlcFunction(b)
	}

	switch h.Function {
	case BvlcFunctionOriginalUnicastNpdu, BvlcFunctionOriginalBroadcastNpdu:
		break
	default:
		return fmt.Errorf("unsupported BVLC function: %d", h.Function)
	}

	if b := buff.Next(2); len(b) == 2 {
		if e := h.BvlcLength.UnmarshalBinary(b); e != nil {
			return nil
		}
	} else {
		return fmt.Errorf("unexpected number of bytes")
	}

	h.NsduLength = h.BvlcLength - 4
	return nil
}
