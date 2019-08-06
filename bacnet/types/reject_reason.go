package types

const (
	RejectReasonOther                    RejectReason = 0
	RejectReasonBufferOverflow                        = 1
	RejectReasonInconsistentParameters                = 2
	RejectReasonInvalidParameterDataType              = 3
	RejectReasonInvalidTag                            = 4
	RejectReasonMissingRequiredParameter              = 5
	RejectReasonParameterOutOfRange                   = 6
	RejectReasonTooManyArguments                      = 7
	RejectReasonUndefinedEnumeration                  = 8
	RejectReasonUnrecognizedService                   = 9
	RejectReasonMax                                   = 10
)

type RejectReason uint8

func (r RejectReason) String() string {
	switch r {
	case RejectReasonBufferOverflow:
		return "buffer overflow"
	case RejectReasonInconsistentParameters:
		return "inconsistent parameters"
	case RejectReasonInvalidParameterDataType:
		return "invalid parameter data type"
	case RejectReasonInvalidTag:
		return "invalid tag"
	case RejectReasonMissingRequiredParameter:
		return "missing required parameter"
	case RejectReasonParameterOutOfRange:
		return "parameter out of range"
	case RejectReasonTooManyArguments:
		return "too many arguments"
	case RejectReasonUndefinedEnumeration:
		return "undefined enumeration"
	case RejectReasonUnrecognizedService:
		return "unrecognized service"
	default:
		return "unknown reason"
	}
}
