package service

import (
	"github.com/zyra/bacnet-2/pkg/object"
	_type "github.com/zyra/bacnet-2/pkg/type"
)

type readPropertyRequest struct {
	*baseService
	propertyId _type.PropertyId
	dest       interface{}
}

func SendReadPropertyRequest(device *object.Device, propertyId _type.PropertyId, dest *object.Property) error {
	s, e := newBaseService(device.OriginInterface)

	if e != nil {
		return e
	}

	req := &readPropertyRequest{
		baseService: s,
		dest:        dest,
		propertyId:  propertyId,
	}



	return nil
}
