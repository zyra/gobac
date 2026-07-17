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

	// Timeout is retained for backwards compatibility but is no longer
	// consulted by MarshalBinary; use Lifetime + HasLifetime instead.
	Timeout uint32

	// IssueConfirmed selects confirmed (true) vs unconfirmed (false) COV
	// notifications. It is only written to the wire ([2]) when Cancel is
	// false.
	IssueConfirmed bool

	// Lifetime is the requested subscription lifetime in seconds. It is
	// only written to the wire ([3]) when HasLifetime is true and Cancel
	// is false.
	Lifetime uint32

	// HasLifetime controls whether the lifetime ([3]) tag is written.
	// When Cancel is true, [2] and [3] are always omitted regardless of
	// this flag, producing the cancellation form of the request.
	HasLifetime bool
}

func (p *SubscribeCov) MarshalBinary() ([]byte, error) {
	if p.ObjectId == nil {
		return nil, errors.New("COV subscription object identifier is required")
	}
	buff := buffPool.Get().(*bytes.Buffer)
	defer buff.Reset()
	defer buffPool.Put(buff)

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
		// issueConfirmedNotifications: Boolean, context tag [2]
		t.TagNumber = 2
		t.LenValue = 1
		if _, err := buff.Write(t.EncodeContextTag()); err != nil {
			return nil, err
		}

		confirmedByte := byte(0)
		if p.IssueConfirmed {
			confirmedByte = 1
		}
		if err := buff.WriteByte(confirmedByte); err != nil {
			return nil, err
		}

		if p.HasLifetime {
			// lifetime: Unsigned seconds, context tag [3]
			lifetime := types.EncodeVarUint(p.Lifetime)
			t.TagNumber = 3
			t.LenValue = len(lifetime)

			if _, err := buff.Write(t.EncodeContextTag()); err != nil {
				return nil, err
			}

			if _, err := buff.Write(lifetime); err != nil {
				return nil, err
			}
		}
	}

	return buff.Bytes(), nil
}

// UnmarshalBinary decodes a SubscribeCOV-Request. Absence of both [2] and
// [3] after the monitored object identifier is the cancellation form; when
// present, [2] and [3] are each independently optional.
func (p *SubscribeCov) UnmarshalBinary(b []byte) error {
	*p = SubscribeCov{}
	offset := 0
	tag := &types.Tag{}

	headerLen := tag.DecodeTag(b[offset:])
	if headerLen == 0 || !tag.IsContext(0) || tag.Opening || tag.Closing {
		return errors.New("expected SubscribeCOV process identifier tag")
	}
	offset += headerLen
	if tag.LenValue < 1 || offset+tag.LenValue > len(b) {
		return errors.New("SubscribeCOV process identifier value is truncated")
	}
	p.ProcessIdentifier32 = types.DecodeVarUint(b[offset : offset+tag.LenValue])
	p.ProcessIdentifier = uint8(p.ProcessIdentifier32)
	offset += tag.LenValue

	headerLen = tag.DecodeTag(b[offset:])
	if headerLen == 0 || !tag.IsContext(1) || tag.Opening || tag.Closing || tag.LenValue != 4 {
		return errors.New("expected SubscribeCOV monitored object identifier tag")
	}
	offset += headerLen
	if offset+4 > len(b) {
		return errors.New("SubscribeCOV monitored object identifier is truncated")
	}
	p.ObjectId = &types.ObjectId{}
	if err := p.ObjectId.UnmarshalBinary(b[offset : offset+4]); err != nil {
		return err
	}
	offset += 4

	if offset == len(b) {
		p.Cancel = true
		return nil
	}

	if headerLen = tag.DecodeTag(b[offset:]); headerLen > 0 && tag.IsContext(2) && !tag.Opening && !tag.Closing {
		if tag.LenValue != 1 || offset+headerLen+1 > len(b) {
			return errors.New("invalid SubscribeCOV issueConfirmedNotifications value")
		}
		offset += headerLen
		v := b[offset]
		if v != 0 && v != 1 {
			return errors.New("invalid SubscribeCOV issueConfirmedNotifications value")
		}
		p.IssueConfirmed = v != 0
		offset++
	}

	if offset < len(b) {
		headerLen = tag.DecodeTag(b[offset:])
		if headerLen == 0 || !tag.IsContext(3) || tag.Opening || tag.Closing {
			return errors.New("expected SubscribeCOV lifetime tag")
		}
		offset += headerLen
		if tag.LenValue < 1 || offset+tag.LenValue > len(b) {
			return errors.New("SubscribeCOV lifetime value is truncated")
		}
		p.Lifetime = types.DecodeVarUint(b[offset : offset+tag.LenValue])
		p.HasLifetime = true
		offset += tag.LenValue
	}

	if offset != len(b) {
		return errors.New("unexpected trailing SubscribeCOV data")
	}

	return nil
}
