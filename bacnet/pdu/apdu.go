package pdu

import (
	"bytes"
	"encoding"
	"errors"
	"fmt"
	"net"

	"github.com/zyra/gobac/bacnet/types"
)

type Apdu struct {
	PduType                   types.PduType
	InvokeID                  uint8
	MaxSegments               types.MaxSegments
	MaxApdu                   types.MaxApduValue
	Segmented                 bool
	MoreFollows               bool
	SegmentedResponseAccepted bool
	Server                    bool
	SequenceNumber            uint8
	ProposedWindowSize        uint8

	ServiceChoice uint8
	Failed        bool

	RequestData  encoding.BinaryMarshaler
	ResponseData encoding.BinaryUnmarshaler
	Payload      []byte

	Errored    bool
	ErrorClass types.ErrorClass
	ErrorCode  types.ErrorCode

	Aborted     bool
	AbortReason types.AbortReason

	Rejected     bool
	RejectReason types.RejectReason

	SenderIP   net.IP
	BacnetPort uint16
}

func (a *Apdu) Reset() {
	*a = Apdu{}
}

func (a *Apdu) GetPduType() uint8 {
	return uint8(a.PduType)
}

func (a *Apdu) SetPduType(t uint8) {
	a.PduType = types.PduType(t)
}

func maxSegmentsCode(value types.MaxSegments) (byte, error) {
	switch value {
	case 0:
		return 0, nil
	case 2:
		return 1, nil
	case 4:
		return 2, nil
	case 8:
		return 3, nil
	case 16:
		return 4, nil
	case 32:
		return 5, nil
	case 64:
		return 6, nil
	case 65:
		return 7, nil
	default:
		return 0, fmt.Errorf("invalid max segments value: %d", value)
	}
}

func maxApduCode(value types.MaxApduValue) (byte, error) {
	switch value {
	case 50:
		return 0, nil
	case 128:
		return 1, nil
	case 206:
		return 2, nil
	case 480:
		return 3, nil
	case 1024:
		return 4, nil
	case 1476:
		return 5, nil
	default:
		return 0, fmt.Errorf("invalid max APDU value: %d", value)
	}
}

func (a *Apdu) MarshalBinary() ([]byte, error) {
	buff := bytes.NewBuffer(nil)
	pduType := a.PduType & 0xf0
	first := byte(a.PduType)
	if a.Segmented {
		first |= types.BIT3
	}
	if a.MoreFollows {
		first |= types.BIT2
	}

	switch pduType {
	case types.PduTypeConfirmedServiceRequest:
		if a.Segmented {
			return nil, errors.New("segmented messages are not supported")
		}
		if a.SegmentedResponseAccepted {
			first |= types.BIT1
		}
		segments, err := maxSegmentsCode(a.MaxSegments)
		if err != nil {
			return nil, err
		}
		maxApdu, err := maxApduCode(a.MaxApdu)
		if err != nil {
			return nil, err
		}
		buff.WriteByte(first)
		buff.WriteByte(segments<<4 | maxApdu)
		buff.WriteByte(a.InvokeID)
		buff.WriteByte(a.ServiceChoice)
	case types.PduTypeUnconfirmedServiceRequest:
		buff.WriteByte(first)
		buff.WriteByte(a.ServiceChoice)
	case types.PduTypeSimpleAck:
		buff.WriteByte(first)
		buff.WriteByte(a.InvokeID)
		buff.WriteByte(a.ServiceChoice)
	case types.PduTypeComplexAck:
		if a.Segmented {
			return nil, errors.New("segmented messages are not supported")
		}
		buff.WriteByte(first)
		buff.WriteByte(a.InvokeID)
		buff.WriteByte(a.ServiceChoice)
	case types.PduTypeError:
		buff.WriteByte(first)
		buff.WriteByte(a.InvokeID)
		buff.WriteByte(a.ServiceChoice)
		for _, value := range []uint32{uint32(a.ErrorClass), uint32(a.ErrorCode)} {
			encoded := types.EncodeVarUint(value)
			tag := types.Tag{TagNumber: types.TagEnumerated, LenValue: len(encoded)}
			buff.Write(tag.EncodeTag())
			buff.Write(encoded)
		}
	case types.PduTypeReject:
		buff.WriteByte(first)
		buff.WriteByte(a.InvokeID)
		buff.WriteByte(byte(a.RejectReason))
	case types.PduTypeAbort:
		if a.Server {
			first |= types.BIT0
		}
		buff.WriteByte(first)
		buff.WriteByte(a.InvokeID)
		buff.WriteByte(byte(a.AbortReason))
	default:
		return nil, fmt.Errorf("unsupported pdu type: %d", pduType)
	}

	if pduType == types.PduTypeConfirmedServiceRequest ||
		pduType == types.PduTypeUnconfirmedServiceRequest || pduType == types.PduTypeComplexAck {
		if a.RequestData != nil {
			data, err := a.RequestData.MarshalBinary()
			if err != nil {
				return nil, err
			}
			buff.Write(data)
		} else if len(a.Payload) > 0 {
			buff.Write(a.Payload)
		}
	}

	return buff.Bytes(), nil
}

func (a *Apdu) UnmarshalBinary(b []byte) error {
	if len(b) == 0 {
		return errors.New("APDU is empty")
	}
	senderIP, port := a.SenderIP, a.BacnetPort
	a.Reset()
	a.SenderIP, a.BacnetPort = senderIP, port
	offset := 1
	readByte := func() (byte, error) {
		if offset >= len(b) {
			return 0, errors.New("APDU is truncated")
		}
		value := b[offset]
		offset++
		return value, nil
	}

	first := b[0]
	a.PduType = types.PduType(first & 0xf0)
	switch a.PduType {
	case types.PduTypeUnconfirmedServiceRequest:
		service, err := readByte()
		if err != nil {
			return err
		}
		a.ServiceChoice = service
		a.Payload = append([]byte(nil), b[offset:]...)
		return a.decodeUnconfirmedApdu(b[offset:])

	case types.PduTypeConfirmedServiceRequest:
		a.SegmentedResponseAccepted = first&types.BIT1 != 0
		if first&types.BIT3 != 0 {
			a.Segmented = true
			a.MoreFollows = first&types.BIT2 != 0
		}
		maximums, err := readByte()
		if err != nil {
			return err
		}
		if err := a.MaxSegments.UnmarshalBinary([]byte{maximums}); err != nil {
			return err
		}
		if err := a.MaxApdu.UnmarshalBinary([]byte{maximums}); err != nil {
			return err
		}
		invokeID, err := readByte()
		if err != nil {
			return err
		}
		a.InvokeID = invokeID
		if a.Segmented {
			sequence, err := readByte()
			if err != nil {
				return err
			}
			a.SequenceNumber = sequence
			window, err := readByte()
			if err != nil {
				return err
			}
			a.ProposedWindowSize = window
		}
		service, err := readByte()
		if err != nil {
			return err
		}
		a.ServiceChoice = service
		a.Payload = append([]byte(nil), b[offset:]...)
		if a.Segmented {
			return nil
		}
		return a.decodeConfirmedApdu(b[offset:])

	case types.PduTypeSimpleAck:
		invokeID, err := readByte()
		if err != nil {
			return err
		}
		a.InvokeID = invokeID
		service, err := readByte()
		if err != nil {
			return err
		}
		a.ServiceChoice = service
		return nil

	case types.PduTypeComplexAck:
		invokeID, err := readByte()
		if err != nil {
			return err
		}
		a.InvokeID = invokeID
		if first&types.BIT3 != 0 {
			a.Segmented = true
			a.MoreFollows = first&types.BIT2 != 0
			sequence, err := readByte()
			if err != nil {
				return err
			}
			a.SequenceNumber = sequence
			window, err := readByte()
			if err != nil {
				return err
			}
			a.ProposedWindowSize = window
			a.Failed = true
			return errors.New("segmented messages are not supported")
		}
		service, err := readByte()
		if err != nil {
			return err
		}
		a.ServiceChoice = service
		a.Payload = append([]byte(nil), b[offset:]...)
		return a.decodeConfirmedApdu(b[offset:])

	case types.PduTypeError:
		a.Failed = true
		a.Errored = true
		invokeID, err := readByte()
		if err != nil {
			return err
		}
		a.InvokeID = invokeID
		service, err := readByte()
		if err != nil {
			return err
		}
		a.ServiceChoice = service
		values := []*uint32{new(uint32), new(uint32)}
		for _, value := range values {
			if offset >= len(b) {
				return errors.New("error APDU is truncated")
			}
			tag := &types.Tag{}
			headerLength := tag.DecodeTag(b[offset:])
			if headerLength == 0 || tag.Context || tag.TagNumber != types.TagEnumerated || tag.Opening || tag.Closing || tag.LenValue < 1 || tag.LenValue > 4 {
				return errors.New("error APDU contains an invalid tag")
			}
			if len(b)-offset-headerLength < tag.LenValue {
				return errors.New("error APDU value is truncated")
			}
			offset += headerLength
			*value = types.DecodeVarUint(b[offset : offset+tag.LenValue])
			offset += tag.LenValue
		}
		a.ErrorClass = types.ErrorClass(*values[0])
		a.ErrorCode = types.ErrorCode(*values[1])
		a.Payload = append([]byte(nil), b[offset:]...)
		return nil

	case types.PduTypeReject:
		a.Failed = true
		a.Rejected = true
		invokeID, err := readByte()
		if err != nil {
			return err
		}
		a.InvokeID = invokeID
		reason, err := readByte()
		if err != nil {
			return err
		}
		a.RejectReason = types.RejectReason(reason)
		return nil

	case types.PduTypeAbort:
		a.Failed = true
		a.Aborted = true
		a.Server = first&types.BIT0 != 0
		invokeID, err := readByte()
		if err != nil {
			return err
		}
		a.InvokeID = invokeID
		reason, err := readByte()
		if err != nil {
			return err
		}
		a.AbortReason = types.AbortReason(reason)
		return nil

	case types.PduTypeSegmentAck:
		a.Failed = true
		return errors.New("segmented messages are not supported")
	default:
		return fmt.Errorf("unsupported pdu type: %d", a.PduType)
	}
}

func (a *Apdu) decodeConfirmedApdu(data []byte) error {
	switch a.ServiceChoice {
	case types.ConfirmedServiceReadProperty:
		response := &ReadPropertyPdu{RequireValue: a.PduType == types.PduTypeComplexAck}
		a.ResponseData = response
		return response.UnmarshalBinary(data)
	case types.ConfirmedServiceCovNotification:
		response := &CovNotification{}
		a.ResponseData = response
		return response.UnmarshalBinary(data)
	}
	return nil
}

func (a *Apdu) decodeUnconfirmedApdu(data []byte) error {
	switch a.ServiceChoice {
	case types.UnconfirmedServiceIAm:
		response := types.NewDevice()
		response.IPAddress = a.SenderIP
		response.Port = a.BacnetPort
		a.ResponseData = response
		return response.UnmarshalBinary(data)
	case types.UnconfirmedServiceCovNotification:
		response := &CovNotification{}
		a.ResponseData = response
		return response.UnmarshalBinary(data)
	}
	return nil
}
