package types

const (
	AbortReasonOther                         AbortReason = 0
	AbortReasonBufferOverflow                            = 1
	AbortReasonInvalidApduInThisState                    = 2
	AbortReasonPreemptedByHigherPriorityTask             = 3
	AbortReasonSegmentationNotSupported                  = 4
	AbortReasonSecurityError                             = 5
	AbortReasonInsufficientSecurity                      = 6
	AbortReasonMax                                       = 7
)

type AbortReason uint8

func (r AbortReason) String() string {
	switch r {
	case AbortReasonBufferOverflow:
		return "buffer overflow"
	case AbortReasonInvalidApduInThisState:
		return "invalid APDU in this state"
	case AbortReasonPreemptedByHigherPriorityTask:
		return "preempted by higher priority task"
	case AbortReasonSegmentationNotSupported:
		return "segmentation not supported"
	case AbortReasonSecurityError:
		return "security error"
	case AbortReasonInsufficientSecurity:
		return "insufficient security"
	default:
		return "unknown reason"
	}
}
