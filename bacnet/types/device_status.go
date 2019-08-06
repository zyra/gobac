package types

const (
	DeviceStatusOperational         DeviceStatus = 0
	DeviceStatusOperationalReadOnly              = 1
	DeviceStatusDownloadRequired                 = 2
	DeviceStatusDownloadInProgress               = 3
	DeviceStatusNonOperational                   = 4
	DeviceStatusBackupInProgress                 = 5
	DeviceStatusMax                              = 6
)

type DeviceStatus = Uint32
