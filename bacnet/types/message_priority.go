package types

const (
	MessagePriorityNormal            MessagePriority = 0
	MessagePriorityUrgent                            = 1
	MessagePriorityCriticalEquipment                 = 2
	MessagePriorityLifeSafety                        = 3
)

type MessagePriority uint8
