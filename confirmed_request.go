package gobac

type ConfirmedRequest struct {
	Request
}

func (s *Server) NewConfirmedRequest() ConfirmedRequest {
	req := ConfirmedRequest{
		Request: s.NewRequest(),
	}
	req.IsConfirmed = true
	req.InvokeID = GetInvokeID()
	req.ExpectingReply = true
	req.EncodeNPCI()

	req.err = make(chan error)
	req.data = make(chan *Response)

	return req
}

func (r *ConfirmedRequest) Send() error {
	r.Server.SetConfirmedHandler(r.InvokeID, r.data)
	return r.Request.Send()
}

func (r *ConfirmedRequest) Cleanup() {
	r.closeChans()
	r.Server.RemoveConfirmedHandler(r.InvokeID)
	ReleaseInvokeID(r.InvokeID)
}
