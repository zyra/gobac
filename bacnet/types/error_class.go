package types

const (
	ErrorClassDevice         ErrorClass = 0
	ErrorClassObject                    = 1
	ErrorClassProperty                  = 2
	ErrorClassResources                 = 3
	ErrorClassSecurity                  = 4
	ErrorClassServices                  = 5
	ErrorClassVT                        = 6
	ErrorClassCommunication             = 7
	ErrorClassMax                       = 8
	ErrorClassProprietaryMin            = 64
	ErrorClassProprietaryMax            = 65535
)

type ErrorClass uint32

func (c ErrorClass) MarshalBinary() ([]byte, error) {
	return EncodeVarUint(uint32(c)), nil
}

func (c *ErrorClass) UnmarshalBinary(b []byte) error {
	*c = ErrorClass(DecodeVarUint(b))
	return nil
}

func (c ErrorClass) String() string {
	switch c {
	case ErrorClassDevice:
		return "device"
	case ErrorClassObject:
		return "object"
	case ErrorClassProperty:
		return "property"
	case ErrorClassResources:
		return "resources"
	case ErrorClassSecurity:
		return "security"
	case ErrorClassServices:
		return "services"
	case ErrorClassVT:
		return "vt"
	case ErrorClassCommunication:
		return "communication"
	default:
		return "unknown"
	}
}
