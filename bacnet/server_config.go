package bacnet

import "time"

type ServerConfig struct {
	// Interface name to listen on
	// Defaults to "eno0"
	InterfaceName string
	// Default DefaultTimeout for requests.
	// This value can be overwritten per request.
	// Defaults to 10 seconds.
	DefaultTimeout time.Duration
	// Set this to true to receive errors
	// you must listen to the Error() chan to avoid deadlock
	// Defaults to false.
	ReceiveErrors bool
	// Number of concurrent listeners to run.
	//
	// The actual number of listeners could reach x2 the value provided here
	// if there is a WhoIs request active; since a WhoIs request will be
	// creating a set of broadcast listeners.
	//
	// The concurrency should be tweaked depending on the expected number of
	// devices on the network, or the expected number of concurrent requests.
	// UDP doesn't establish a connection before sending covData, so for example if
	// you Send a WhoIs request to a network with 300 devices and your concurrency
	// is lower than 300, there is a chance that you might miss some of the
	// broadcasts sent by these devices.
	//
	// The number of concurrent requests cannot exceed 255 since we are limited to
	// 255 invoke identifiers for confirmed requests (anything other than WhoIs).
	//
	// Defaults to 10.
	Concurrency uint
	// Server BBMD port.
	// This is the port that we will identify ourselves with when making
	// requests to a BACnet device.
	// Defaults to 0xBAC0 (47808)
	ServerBBMDPort uint16
	// Port that the BACnet is running on.
	// Defaults to 0xBAC0 (47808)
	ListenPort uint16
}

// Creates a new ServerConfig object that is populated with default values
func NewServerConfig() *ServerConfig {
	return &ServerConfig{
		InterfaceName:  "eno0",
		DefaultTimeout: time.Duration(time.Second * 10),
		ReceiveErrors:  false,
		Concurrency:    10,
		ServerBBMDPort: 0xBAC0,
		ListenPort:     0xBAC0,
	}
}

// Convenience method to set InterfaceName
func (s *ServerConfig) SetInterfaceName(name string) *ServerConfig {
	s.InterfaceName = name
	return s
}

// Convenience method to set DefaultTimeout
func (s *ServerConfig) SetDefaultTimeout(timeout time.Duration) *ServerConfig {
	s.DefaultTimeout = timeout
	return s
}

// Convenience method to set ReceiveErrors
func (s *ServerConfig) SetReceiveErrors(receive bool) *ServerConfig {
	s.ReceiveErrors = receive
	return s
}

// Convenience method to set Concurrency
func (s *ServerConfig) SetConcurrency(concurrency uint) *ServerConfig {
	s.Concurrency = concurrency
	return s
}

// Convenience method to set ServerBBMDPort
func (s *ServerConfig) SetServerBBMDPort(port uint16) *ServerConfig {
	s.ServerBBMDPort = port
	return s
}

// Convenience method to set ListenPort
func (s *ServerConfig) SetListenPort(port uint16) *ServerConfig {
	s.ListenPort = port
	return s
}
