package types

import (
	"errors"
	"fmt"
	"math"
)

type ObjectId struct {
	Type     ObjectType
	Instance Uint16

	extendedInstance uint32
}

func (o ObjectId) String() string {
	return fmt.Sprintf("type: %d, instance: %d", o.Type, o.InstanceNumber())
}

// InstanceNumber returns the complete 22-bit BACnet object instance. The
// Instance field is retained for source compatibility with earlier releases.
func (o ObjectId) InstanceNumber() uint32 {
	if o.extendedInstance != 0 && Uint16(o.extendedInstance) == o.Instance {
		return o.extendedInstance
	}
	return uint32(o.Instance)
}

// SetInstanceNumber sets a BACnet object instance without truncating values
// above 65535.
func (o *ObjectId) SetInstanceNumber(instance uint32) error {
	if instance > BacnetMaxInstance {
		return fmt.Errorf("object instance %d exceeds %d", instance, BacnetMaxInstance)
	}
	o.Instance = Uint16(instance)
	if instance > math.MaxUint16 {
		o.extendedInstance = instance
	} else {
		o.extendedInstance = 0
	}
	return nil
}

func (o *ObjectId) MarshalBinary() ([]byte, error) {
	val := uint32(o.Type) & BacnetMaxObject
	val <<= BacnetInstanceBits
	val |= o.InstanceNumber() & BacnetMaxInstance
	return Uint32(val).MarshalBinary()
}

func (o *ObjectId) UnmarshalBinary(b []byte) error {
	if b == nil {
		return errors.New("received a nil byte slice")
	}

	if len(b) != 4 {
		return errors.New("object identifier expects exactly 4 octets")
	}

	v := Uint32(0)

	if e := v.UnmarshalBinary(b); e != nil {
		return e
	}

	o.Type = Uint16((v >> BacnetInstanceBits) & BacnetMaxObject)
	if err := o.SetInstanceNumber(uint32(v) & BacnetMaxInstance); err != nil {
		return err
	}

	return nil
}
