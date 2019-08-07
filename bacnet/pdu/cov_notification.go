package pdu

import (
	"bytes"
	"fmt"
	"github.com/zyra/gobac/bacnet/types"
)

type CovNotification struct {
	ProcessIdentifier uint8
	DeviceObjectId    *types.ObjectId
	ObjectId          *types.ObjectId
	TimeRemaining     uint32
	Properties        []*types.Property
	Priority          uint8
}

func (n *CovNotification) UnmarshalBinary(b []byte) (err error) {
	buff := buffPool.Get().(*bytes.Buffer)
	buff.Write(b)
	defer buff.Reset()
	defer buffPool.Put(buff)
	t := &types.Tag{}

	// decode process id
	buff.Next(t.DecodeTag(buff.Bytes()))

	if !t.IsContext(0) {
		return fmt.Errorf("expected tag %d and got %d\n", 0, t.TagNumber)
	}

	if n.ProcessIdentifier, err = buff.ReadByte(); err != nil {
		return err
	}

	// decode deivce obj id
	buff.Next(t.DecodeTag(buff.Bytes()))

	if !t.IsContext(1) {
		return fmt.Errorf("expected tag %d and got %d\n", 1, t.TagNumber)
	}

	n.DeviceObjectId = &types.ObjectId{}

	if err := n.DeviceObjectId.UnmarshalBinary(buff.Next(t.LenValue)); err != nil {
		return err
	}

	// decode obj id
	buff.Next(t.DecodeTag(buff.Bytes()))

	if !t.IsContext(2) {
		return fmt.Errorf("expected tag %d and got %d\n", 2, t.TagNumber)
	}

	n.ObjectId = &types.ObjectId{}

	if err := n.ObjectId.UnmarshalBinary(buff.Next(t.LenValue)); err != nil {
		return err
	}

	// Decode time remaining
	buff.Next(t.DecodeTag(buff.Bytes()))
	if !t.IsContext(3) {
		return fmt.Errorf("expected tag %d and got %d\n", 3, t.TagNumber)
	}

	n.TimeRemaining = types.DecodeVarUint(buff.Next(t.LenValue))

	// Decode opening tag
	buff.Next(t.DecodeTag(buff.Bytes()))

	if !t.IsContext(4) {
		return fmt.Errorf("expected tag %d and got %d\n", 4, t.TagNumber)
	}

	// read values
	values := make([]*types.Property, 0, 2)

	for buff.Len() > 1 {
		b := buff.Bytes()[0]
		if b&0x07 == 7 {
			// closing tag
			// let's exit loop before Property steals our bytes!
			break
		}

		val := types.Property{
			ObjectId: n.ObjectId,
		}

		if err := val.UnmarshalBinary(buff.Bytes()); err != nil {
			return err
		}

		// mark bytes as read
		buff.Next(val.Length)

		// append prop
		values = append(values, &val)

		// Check optional priority
		r := t.DecodeTag(buff.Bytes())

		if t.IsContext(3) {
			buff.Next(r)
			n.Priority = uint8(t.LenValue)
		}

		// Check closing tag
		r = t.DecodeTag(buff.Bytes())
		if t.IsContext(4) {
			buff.Next(r)
			break
		} else {
			lll := buff.Len()
			if lll == 1 {
				return fmt.Errorf("expected tag %d and got %d\n", 4, t.TagNumber)
			}
			continue
		}
	}

	n.Properties = values

	return nil
}
