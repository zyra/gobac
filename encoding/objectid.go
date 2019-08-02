package encoding

import (
	"github.com/zyra/gobac/types"
)

func (buf *Buffer) EncodeObjectId(objectType types.ObjectType, objectInstance uint16) error {
	octet := uint32(objectType) & types.BACNET_MAX_OBJECT
	octet <<= types.BACNET_INSTANCE_BITS
	octet |= uint32(objectInstance) & types.BACNET_MAX_INSTANCE
	return buf.EncodeUnsigned32(octet)
}

func (buf *Buffer) DecodeObjectId() (objectType types.ObjectType, objectInstance uint16) {
	value := DecodeUnsigned32(buf.Next(4))
	objectType = uint16((value >> types.BACNET_INSTANCE_BITS) & types.BACNET_MAX_OBJECT)
	objectInstance = uint16(value & types.BACNET_MAX_INSTANCE)
	return objectType, objectInstance
}
