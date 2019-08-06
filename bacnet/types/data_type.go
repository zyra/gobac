package types

type DataType = uint8

const (
	TagNull            DataType = 0
	TagBoolean                  = 1
	TagUnsigned                 = 2
	TagSigned                   = 3
	TagReal                     = 4
	TagDouble                   = 5
	TagOctetString              = 6
	TagCharacterString          = 7
	TagBitString                = 8
	TagEnumerated               = 9
	TagDate                     = 10
	TagTime                     = 11
	TagObjectId                 = 12
)
