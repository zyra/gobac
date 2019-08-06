package types

type ApplicationTag = Uint32

const (
	ApplicationTagNull                          ApplicationTag = 0
	ApplicationTagBoolean                                      = 1
	ApplicationTagUnsignedInt                                  = 2
	ApplicationTagSignedInt                                    = 3
	ApplicationTagReal                                         = 4
	ApplicationTagDouble                                       = 5
	ApplicationTagOctetString                                  = 6
	ApplicationTagCharacterString                              = 7
	ApplicationTagBitString                                    = 8
	ApplicationTagEnumerated                                   = 9
	ApplicationTagDate                                         = 10
	ApplicationTagTime                                         = 11
	ApplicationTagObjectId                                     = 12
	ApplicationTagReserve1                                     = 13
	ApplicationTagReserve2                                     = 14
	ApplicationTagReserve3                                     = 15
	ApplicationTagMax                                          = 16
	ApplicationTagEmptylist                                    = 17
	ApplicationTagWeeknday                                     = 18
	ApplicationTagDaterange                                    = 19
	ApplicationTagDatetime                                     = 20
	ApplicationTagTimestamp                                    = 21
	ApplicationTagError                                        = 22
	ApplicationTagDeviceObjectPropertyReference                = 23
	ApplicationTagDeviceObjectReference                        = 24
	ApplicationTagObjectPropertyReference                      = 25
	ApplicationTagDestination                                  = 26
	ApplicationTagRecipient                                    = 27
	ApplicationTagCovSubscription                              = 28
	ApplicationTagCalendarEntry                                = 29
	ApplicationTagWeeklySchedule                               = 30
	ApplicationTagSpecialEvent                                 = 31
	ApplicationTagReadAccessSpecification                      = 32
	ApplicationTagLightingCommand                              = 33
)
