package gobac

import "errors"

type covRequest struct {
	readPropertyRequest
	out            chan<- *Property
	err            chan<- error
	done           chan int
	subscriptionId uint32
}

func (s *Server) SendCovRequest(object *Object,
	subscriptionId uint32,
	out chan<- *Property,
	err chan<- error,
	done chan int) {
	req := &covRequest{
		out:            out,
		err:            err,
		done:           done,
		subscriptionId: subscriptionId,
	}

	req.Request = NewRequest(s)
	req.object = object

	req.InvokeID = NewTransaction()

	req.ExpectingReply = true
	req.IsBroadcastTarget = false
	req.EncodeNpdu()

	if e := req.EncodeCovSubscribeApdu(); e != nil {
		err <- e
		done <- 1
		return
	}

	if req.Len() >= int(object.Device.MaxAPDU) {
		err <- errors.New("apdu too large")
		done <- 1
		return
	}

	req.Target = object.Device.IPAddress
	c, h := getChanHandler()
	s.setConfirmedHandler(req.InvokeID, h)
	req.Send()
	go req.startListening(c)
}

func (r *covRequest) startListening(c <-chan *Response) {
	defer ReleaseTransaction(r.InvokeID)
	defer r.Server.removeConfirmedHandler(r.InvokeID)
	for {
		data := <-c
		if data.Failed {
			r.done <- 1
			r.err <- errors.New("device rejected request")
			break
		} else if data.PduType == PduTypeSimpleAck {
			// dismiss
		} else {
			go r.emitData(data)
		}

		continue
	}
}

func (r *covRequest) emitData(data *Response) {
	prop := Property{}

	err := data.DecodeReadPropertyApdu(r.object, r.propertyId, &prop)

	if err != nil {
		r.err <- err
	} else {
		r.out <- &prop
	}
}

func (r *covRequest) EncodeCovSubscribeApdu() (err error) {
	err = r.AppendByte(PduTypeConfirmedServiceRequest)
	err = r.AppendByte(5)
	err = r.AppendByte(r.InvokeID)
	err = r.AppendByte(ConfirmedServiceSubscribeCov)

	err = r.EncodeTag(0, true, getUnsignedLen(uint(r.subscriptionId)))
	err = r.EncodeUnsigned(r.subscriptionId) // subscription ID

	// monitored object
	err = r.EncodeTag(1, true, 4)
	err = r.EncodeObjectId(r.object.Type, r.object.Instance)

	err = r.EncodeTag(2, true, 1)
	err = r.AppendByte(1)

	err = r.EncodeTag(3, true, 4)
	err = r.EncodeUnsigned32(0)
	return err
}
