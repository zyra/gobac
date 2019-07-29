package encoding

import (
	"bytes"
	_type "github.com/zyra/gobac/types"
)

func DecodeObjectId(buf *bytes.Buffer) (objectType uint32, objectInstance uint32) {
	value := DecodeUnsigned32(buf.Next(4))
	objectType = uint32(uint16((value >> _type.BACNET_INSTANCE_BITS) & _type.BACNET_MAX_OBJECT))
	objectInstance = value & _type.BACNET_MAX_INSTANCE
	return objectType, objectInstance
}
