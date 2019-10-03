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
	// server BBMD port.
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
