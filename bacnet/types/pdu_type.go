package types

type PduType uint8

const (
	PduTypeConfirmedServiceRequest   PduType = 0
	PduTypeUnconfirmedServiceRequest         = 0x10
	PduTypeSimpleAck                         = 0x20
	PduTypeComplexAck                        = 0x30
	PduTypeSegmentAck                        = 0x40
	PduTypeError                             = 0x50
	PduTypeReject                            = 0x60
	PduTypeAbort                             = 0x70
)
