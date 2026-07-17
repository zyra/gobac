package bacnet

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/zyra/gobac/v2/bacnet/pdu"
	"github.com/zyra/gobac/v2/bacnet/types"
)

// WhoHasQuery selects the object a Who-Has broadcast searches for. Exactly
// one of ObjectId / ObjectName must be set: a non-empty ObjectName selects
// the by-name form, otherwise ObjectId must be non-nil. LowLimit/HighLimit
// restrict replies to devices whose instance falls within the range when
// HasRange is true.
type WhoHasQuery struct {
	LowLimit, HighLimit uint32
	HasRange            bool
	ObjectId            *types.ObjectId
	ObjectName          string
}

// IHaveResult is one I-Have reply collected in response to a Who-Has
// broadcast.
type IHaveResult struct {
	Device     net.IP         // sender address
	DeviceId   types.ObjectId // device object identifier
	ObjectId   types.ObjectId // found object
	ObjectName string
}

// WhoHas broadcasts a Who-Has request and returns a channel of I-Have
// replies collected until timeout elapses (or ctx is done, if earlier).
func (s *Server) WhoHas(ctx context.Context, timeout time.Duration, query WhoHasQuery) (<-chan *IHaveResult, error) {
	hasObjectId := query.ObjectId != nil
	hasObjectName := query.ObjectName != ""
	if hasObjectId == hasObjectName {
		return nil, errors.New("exactly one of ObjectId or ObjectName must be set")
	}

	if ctx == nil {
		ctx = context.Background()
	}
	bCtx, cancel := context.WithTimeout(ctx, timeout)

	req := NewRequest()
	req.SetUnconfirmedService(types.UnconfirmedServiceWhoHas, &pdu.WhoHas{
		LowLimit:   query.LowLimit,
		HighLimit:  query.HighLimit,
		HasRange:   query.HasRange,
		ObjectId:   query.ObjectId,
		ObjectName: query.ObjectName,
	})

	if err := req.Broadcast(s, types.UnconfirmedServiceIHave); err != nil {
		cancel()
		req.Release()
		return nil, err
	}

	resultCh := make(chan *IHaveResult, 10)

	go func() {
		defer close(resultCh)
		defer cancel()
		defer req.Release()
		for {
			select {
			case <-bCtx.Done():
				return

			case data := <-req.Data():
				if data == nil {
					continue
				}
				if data.Successful() {
					if val, ok := data.ResponseData().(*pdu.IHave); ok {
						result := &IHaveResult{
							Device:     data.Apdu.SenderIP,
							DeviceId:   val.DeviceId,
							ObjectId:   val.ObjectId,
							ObjectName: val.ObjectName,
						}
						select {
						case resultCh <- result:
						case <-bCtx.Done():
							data.Release()
							return
						}
					}
				}
				data.Release()
			}
		}
	}()

	return resultCh, nil
}
