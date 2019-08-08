package pdu

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"github.com/zyra/gobac/bacnet/types"
	"net"
)

type Apdu struct {
	PduType            types.PduType
	InvokeID           uint8
	MaxSegments        types.MaxSegments
	MaxApdu            types.MaxApduValue
	Segmented          bool
	MoreFollows        bool
	SequenceNumber     uint8
	ProposedWindowSize uint8

	ServiceChoice uint8
	Failed        bool

	RequestData  encoding.BinaryMarshaler
	ResponseData encoding.BinaryUnmarshaler

	Errored    bool
	ErrorClass types.ErrorClass
	ErrorCode  types.ErrorCode

	Aborted     bool
	AbortReason types.AbortReason

	Rejected     bool
	RejectReason types.RejectReason

	SenderIP *net.IP
}

func (a *Apdu) Reset() {
	a.PduType = 0
	a.InvokeID = 0
	a.MaxSegments = 0
	a.MaxApdu = 0
	a.Segmented = false
	//a.MoreFollows = false
	//a.SequenceNumber = 0
	//a.ProposedWindowSize = 0
	a.ServiceChoice = 0
	a.Failed = false

	a.RequestData = nil
	a.ResponseData = nil

	a.Errored = false
	a.Aborted = false
	a.Rejected = false
	a.SenderIP = nil
}

func (a *Apdu) GetPduType() uint8 {
	return uint8(a.PduType)
}

func (a *Apdu) SetPduType(t uint8) {
	a.PduType = types.PduType(t)
}

func (a *Apdu) MarshalBinary() (b []byte, e error) {
	buff := buffPool.Get().(*bytes.Buffer)
	buff.WriteByte(byte(a.PduType))
	defer buff.Reset()
	defer buffPool.Put(buff)

	// write invoke ID if we have one
	if a.InvokeID > 0 {
		// Write max segments & max APDU
		// TODO change this if segmentation becomes supported
		e = buff.WriteByte(5)

		e = buff.WriteByte(byte(a.InvokeID))
	}

	// write service choice
	e = buff.WriteByte(byte(a.ServiceChoice))

	if a.RequestData != nil {
		if b, e := a.RequestData.MarshalBinary(); e != nil {
			return nil, e
		} else {
			_, e = buff.Write(b)
		}
	}

	if e != nil {
		return nil, e
	}

	return buff.Bytes(), e
}

func (a *Apdu) UnmarshalBinary(b []byte) (e error) {
	buff := buffPool.Get().(*bytes.Buffer)
	buff.Write(b)
	defer buff.Reset()
	defer buffPool.Put(buff)

	magicByte, e := buff.ReadByte()

	if e != nil {
		return e
	}

	a.PduType = types.PduType(magicByte & 0xF0)

	switch a.PduType {
	case types.PduTypeUnconfirmedServiceRequest:
		defer a.decodeUnconfirmedApdu(buff, &e)
		break

	case types.PduTypeSegmentAck:
		a.Failed = true
		return errors.New("segmented messages are not supported")

	case types.PduTypeConfirmedServiceRequest:
		if segmented := magicByte & types.BIT3; segmented != 0 {
			a.Segmented = true

			if x := magicByte & types.BIT2; x != 0 {
				a.MoreFollows = true
			}
			a.Failed = true
			return errors.New("segmented messages are not supported")
		}

		magicByte2, err := buff.ReadByte()

		if err != nil {
			return err
		}

		if e := a.MaxSegments.UnmarshalBinary([]byte{magicByte2}); e != nil {
			return e
		}

		if e := a.MaxApdu.UnmarshalBinary([]byte{magicByte2}); e != nil {
			return e
		}

		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			a.InvokeID = b
		}
		defer a.decodeConfirmedApdu(buff, &e)
		break

	case types.PduTypeSimpleAck:
		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			a.InvokeID = b
		}
		break

	case types.PduTypeComplexAck:
		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			a.InvokeID = b
		}

		if segmented := magicByte & types.BIT3; segmented != 0 {
			a.Segmented = true

			if b, e := buff.ReadByte(); e != nil {
				return e
			} else {
				a.SequenceNumber = b
			}

			if b, e := buff.ReadByte(); e != nil {
				return e
			} else {
				a.ProposedWindowSize = b
			}

			a.Failed = true
			return errors.New("segmented messages are not supported")
		}

		defer a.decodeConfirmedApdu(buff, &e)
		break

	case types.PduTypeError:
		a.Failed = true
		a.Errored = true

		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			a.InvokeID = b
		}

		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			a.ServiceChoice = b
		}

		t := &types.Tag{}

		buff.Next(t.DecodeTag(buff.Bytes()))

		if e := a.ErrorClass.UnmarshalBinary(buff.Next(t.LenValue)); e != nil {
			return e
		}

		buff.Next(t.DecodeTag(buff.Bytes()))

		if e := a.ErrorCode.UnmarshalBinary(buff.Next(t.LenValue)); e != nil {
			return e
		}
		return nil

	case types.PduTypeReject:
		a.Failed = true
		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			a.InvokeID = uint8(b)
		}

		a.Rejected = true

		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			a.RejectReason = types.RejectReason(b)
		}

		if buff.Len() == 0 {
			return nil
		}
		break

	case types.PduTypeAbort:
		a.Failed = true

		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			a.InvokeID = uint8(b & types.BIT0)
		}

		a.Aborted = true

		if b, e := buff.ReadByte(); e != nil {
			return e
		} else {
			a.AbortReason = types.AbortReason(b)
		}
		break

	default:
		return fmt.Errorf("unsupported pdu type: %d", a.PduType)
	}

	if b, e := buff.ReadByte(); e != nil {
		return e
	} else {
		a.ServiceChoice = uint8(b)
	}

	return e
}

func (a *Apdu) decodeConfirmedApdu(buff *bytes.Buffer, err *error) {
	switch a.ServiceChoice {
	case types.ConfirmedServiceReadProperty:
		res := &ReadPropertyPdu{}
		a.ResponseData = res
		*err = a.ResponseData.UnmarshalBinary(buff.Bytes())
		return

	case types.ConfirmedServiceCovNotification:
		res := &CovNotification{}
		a.ResponseData = res
		*err = a.ResponseData.UnmarshalBinary(buff.Bytes())
		return
	}
}

func (a *Apdu) decodeUnconfirmedApdu(buff *bytes.Buffer, err *error) {
	switch a.ServiceChoice {
	case types.UnconfirmedServiceIAm:
		res := types.NewDevice()
		res.IPAddress = a.SenderIP
		a.ResponseData = res
		*err = res.UnmarshalBinary(buff.Bytes())
		return
	}
}
