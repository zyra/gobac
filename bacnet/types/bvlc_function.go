package types

type BvlcFunction uint8

const (
	BvlcFunctionResult                          BvlcFunction = 0
	BvlcFunctionWriteBroadcastDistributionTable              = 1
	BvlcFunctionReadBroadcastDistTable                       = 2
	BvlcFunctionReadBroadcastDistTableAck                    = 3
	BvlcFunctionForwardedNpdu                                = 4
	BvlcFunctionRegisterForeignDevice                        = 5
	BvlcFunctionReadForeignDeviceTable                       = 6
	BvlcFunctionReadForeignDeviceTableAck                    = 7
	BvlcFunctionDeleteForeignDeviceTableEntry                = 8
	BvlcFunctionDistributeBroadcastToNetwork                 = 9
	BvlcFunctionOriginalUnicastNpdu                          = 10
	BvlcFunctionOriginalBroadcastNpdu                        = 11
	BvlcFunctionSecureBvll                                   = 12
	BvlcFunctionMax                                          = 13
)
