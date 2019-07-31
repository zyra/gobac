package gobac

import (
	"errors"
	"fmt"
	"github.com/zyra/bacnet"
	"github.com/zyra/gobac/types"
	"sync"
	"time"
)

type whoIsRequest struct {
	*Request
	devices   *[]*Device
	mutex     sync.RWMutex
	waitGroup sync.WaitGroup
}

func (s *Server) WhoIs(dest *[]*Device) error {
	req := &whoIsRequest{
		devices: dest,
		Request: NewRequest(s),
	}
	req.EncodeNpdu()
	req.EncodeWhoIsApdu()
	tc, c, h := getChanHandlerWithTimeout(time.Second * 5)
	s.setUnconfirmedHandler(UnconfirmedServiceIAm, h)
	defer s.removeUnconfirmedHandler(UnconfirmedServiceIAm)
	defer req.waitGroup.Wait()

	req.Send()

Loop:
	for {
		select {
		case <-tc:
			break Loop
		case data := <-c:
			req.waitGroup.Add(1)
			go req.handle(data)
			continue
		}
	}

	return nil
}

func (r *whoIsRequest) handle(v *Response) {
	defer r.waitGroup.Done()
	device := NewDevice()
	device.Server = r.Server

	if err := v.DecodeIAmApdu(device); err != nil {
		fmt.Println("error decoding response", err)
	} else {
		device.OriginInterface = r.Server.InterfaceName
		device.IPAddress = &v.Sender.IP
		r.mutex.Lock()
		defer r.mutex.Unlock()
		*r.devices = append(*r.devices, device)
	}
}

func (d *Request) EncodeWhoIsApdu() {
	_ = d.AppendBytes([]byte{
		PduTypeUnconfirmedServiceRequest,
		UnconfirmedServiceWhoIs,
	})

	//d.EncodeContext(0, minInstance)
	//d.EncodeContext(1, maxInstance)
}

func (r *Response) DecodeIAmApdu(dest *Device) error {
	tagNumber, lenValue := r.DecodeTag()

	if tagNumber != bacnet.ApplicationTag_OBJECT_ID {
		return errors.New("invalid object id application tag")
	}

	// Object type & instance
	objectType, objectInstance := r.DecodeObjectId()

	if objectType != bacnet.OBJECT_DEVICE {
		return errors.New("object type isn't a device")
	}

	dest.Type = objectType
	dest.Instance = objectInstance

	// Max APDU
	tagNumber, lenValue = r.DecodeTag()

	if tagNumber != bacnet.ApplicationTag_UNSIGNED_INT {
		return errors.New("invalid max apdu application tag")
	}

	dest.MaxAPDU = r.DecodeUnsigned(lenValue)

	// Segmentation
	tagNumber, lenValue = r.DecodeTag()

	if tagNumber != types.ApplicationTag_ENUMERATED {
		return errors.New("invalid segmentation application tag")
	}

	segmentation := r.DecodeUnsigned(lenValue)

	if segmentation >= types.MAX_BACNET_SEGMENTATION {
		return errors.New("invalid segmentation value")
	}

	dest.Segmentation = uint8(segmentation)

	// Vendor ID
	tagNumber, lenValue = r.DecodeTag()

	if tagNumber != types.ApplicationTag_UNSIGNED_INT {
		return errors.New("invalid vendor id application tag")
	}

	vendorId := r.DecodeUnsigned(lenValue)

	if vendorId > 0xFFFF {
		return errors.New("invalid vendor id")
	}

	dest.VendorID = uint16(vendorId)

	return nil
}
