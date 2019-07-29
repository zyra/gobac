package gobac

import (
	"sync"
)

type Rcv struct {
}

type serviceRequest struct {
	mutex              sync.RWMutex
	waitGroup          sync.WaitGroup
	request            *Request
	transaction        *Transaction
	TransactionHandler TransactionHandler
	server             *Server
}

func newServiceRequest(s *Server) *serviceRequest {
	req := NewRequest(s)
	req.EncodeNpdu()
	return &serviceRequest{
		request: req,
		server:  s,
	}
}

func newConfirmedServiceRequest(s *Server, handler TransactionHandler) *serviceRequest {
	sr := newServiceRequest(s)
	sr.transaction = NewTransaction(handler)
	return sr
}
