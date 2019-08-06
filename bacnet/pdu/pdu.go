package pdu

type RequestPdu interface {
	MarshalBinary() ([]byte, error)
	GetPduType() uint8
}

type ResponsePdu interface {
	UnmarshalBinary([]byte) error
	SetPduType(t uint8)
}
