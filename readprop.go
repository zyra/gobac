package gobac

import (
	"github.com/zyra/gobac/types"
)

type readPropertyRequest struct {
	*serviceRequest
	propertyId types.PropertyId
	dest       interface{}
}

//func SendReadPropertyRequest(device *Device, propertyId types.PropertyId, dest *Property) error {
//	s := newServiceRequest()
//
//	if e != nil {
//		return e
//	}
//
//	req := &readPropertyRequest{
//		baseService: s,
//		dest:        dest,
//		propertyId:  propertyId,
//	}
//
//	fmt.Println(req)
//
//	return nil
//}
