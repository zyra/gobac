package gobac

import (
	"context"
	"time"
)

type UnconfirmedRequest struct {
	Request
}

func (s *Server) NewUnconfirmedRequest() UnconfirmedRequest {
	req := UnconfirmedRequest{
		Request: s.NewRequest(),
	}
	req.EncodeNPCI()

	// Make a big chan just in case.
	// This should be configurable/variable
	// if we end up doing an unconfirmed
	// request other than WhoIs.
	// But for now it should be fine...
	req.Request.data = make(chan *Response, 128)
	req.Request.err = make(chan error, 128)

	return req
}

func (req *UnconfirmedRequest) Send(ctx context.Context) error {
	req.Server.SetUnconfirmedHandler(req.ServiceChoice, req.Request.data)
	req.Server.ReceiveBroadcast(ctx)

	// Sleep for a bit until our listeners are up
	time.Sleep(time.Second)

	println("sending req now")
	err := req.Request.Send()

	if err != nil {
		return err
	}

	return nil
}

func (req *UnconfirmedRequest) Cleanup() {
	req.closeChans()
	req.Server.RemoveUnconfirmedHandler(req.ServiceChoice)
}
