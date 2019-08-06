package types

type ConfirmedService = uint8

const (
	ConfirmedServiceReadProperty    ConfirmedService = 12
	ConfirmedServiceWriteProperty                    = 15
	ConfirmedServiceSubscribeCov                     = 5
	ConfirmedServiceCovNotification                  = 1
)

type UnconfirmedService = uint8

const (
	UnconfirmedServiceIAm             UnconfirmedService = 0
	UnconfirmedServiceCovNotification                    = 0x2
	UnconfirmedServiceWhoIs                              = 0x8
)
