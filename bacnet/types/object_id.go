package types

import (
	"errors"
	"fmt"
)

type ObjectId struct {
	Type     ObjectType
	Instance Uint16
}

func (o ObjectId) String() string {
	return fmt.Sprintf("type: %d, instance: %d", o.Type, o.Instance)
}

func (o *ObjectId) MarshalBinary() ([]byte, error) {
	val := uint32(o.Type) & BacnetMaxObject
	val <<= BacnetInstanceBits
	val |= uint32(o.Instance) & BacnetMaxInstance
	return Uint32(val).MarshalBinary()
}

func (o *ObjectId) UnmarshalBinary(b []byte) error {
	if b == nil {
		return errors.New("received a nil byte slice")
	}

	if len(b) == 0 {
		return errors.New("received an empty byte slice")
	}

	v := Uint32(0)

	if e := v.UnmarshalBinary(b); e != nil {
		return e
	}

	o.Type = Uint16((v >> BacnetInstanceBits) & BacnetMaxObject)
	o.Instance = Uint16(v & BacnetMaxInstance)

	return nil
}
