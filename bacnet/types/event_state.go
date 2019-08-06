package types

type EventState = Uint32

const (
	EventStateNormal    EventState = 0
	EventStateFault                = 1
	EventStateOffnormal            = 2
	EventStateHighLimit            = 3
	EventStateLowLimit             = 4
)
