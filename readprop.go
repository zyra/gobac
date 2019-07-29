package gobac

import (
	"fmt"
	"github.com/zyra/gobac/types"
)

type readPropertyRequest struct {
	*baseService
	propertyId types.PropertyId
	dest       interface{}
}

func SendReadPropertyRequest(device *Device, propertyId types.PropertyId, dest *Property) error {
	s, e := newBaseService(device.OriginInterface)

	if e != nil {
		return e
	}

	req := &readPropertyRequest{
		baseService: s,
		dest:        dest,
		propertyId:  propertyId,
	}

	fmt.Println(req)

	return nil
}
