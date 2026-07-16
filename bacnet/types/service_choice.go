package types

type ConfirmedService = uint8

const (
	ConfirmedServiceReadProperty         ConfirmedService = 12
	ConfirmedServiceReadPropertyMultiple ConfirmedService = 14
	ConfirmedServiceWriteProperty        ConfirmedService = 15
	ConfirmedServiceSubscribeCov         ConfirmedService = 5
	ConfirmedServiceCovNotification      ConfirmedService = 1
)

type UnconfirmedService = uint8

const (
	UnconfirmedServiceIAm             UnconfirmedService = 0
	UnconfirmedServiceCovNotification UnconfirmedService = 0x2
	UnconfirmedServiceWhoIs           UnconfirmedService = 0x8
)
