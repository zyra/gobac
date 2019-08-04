package gobac

import (
	"fmt"
	"github.com/zyra/gobac/types"
)

type CovNotification struct {
	ProcessIdentifier uint8
	DeviceID          uint16
	ObjectType        types.ObjectType
	ObjectInstance    uint16
	TimeRemaining     uint32
	Properties        *[]PropertyValue
}

func (r *Response) DecodeCovNotification() error {
	dest := &CovNotification{}
	r.Dest = dest
	t, v := r.DecodeTag()

	if t != 0 {
		return fmt.Errorf("expected tag %d and got %d\n", 0, t)
	}

	dest.ProcessIdentifier = r.NextOne()

	fmt.Println("process id is ", dest.ProcessIdentifier)

	t, v = r.DecodeTag()

	if t != 1 {
		return fmt.Errorf("expected tag %d and got %d\n", 1, t)
	}

	objectType, objectInstance := r.DecodeObjectId()

	fmt.Println("obj instance is ", objectInstance)

	if objectType != 8 {
		return fmt.Errorf("expected objectType to be 8 (device) but got %d\n", objectType)
	}

	dest.DeviceID = objectInstance

	t, v = r.DecodeTag()

	if t != 2 {
		return fmt.Errorf("expected tag %d and got %d\n", 2, t)
	}

	dest.ObjectType, dest.ObjectInstance = r.DecodeObjectId()

	t, v = r.DecodeTag()

	if t != 3 {
		return fmt.Errorf("expected tag %d and got %d\n", 3, t)
	}

	// Time remaining
	dest.TimeRemaining = r.DecodeUnsigned(v)

	t, v = r.DecodeTag()

	if t != 4 {
		return fmt.Errorf("expected tag %d and got %d\n", 4, t)
	}

	values := make([]PropertyValue, 2)
	for r.Len() > 1 {
		value := r.DecodePropertyValue()
		values = append(values, value)
	}

	dest.Properties = &values

	return nil
}
