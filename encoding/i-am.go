package encoding

import (
	"bytes"
	"github.com/kataras/iris/core/errors"
	"github.com/zyra/bacnet"
	"github.com/zyra/gobac"
	"github.com/zyra/gobac/types"
)

type IAmServiceRequest []byte

func (r *IAmServiceRequest) Decode(dest *gobac.Device) error {
	buf := bytes.NewBuffer(*r)
	tagNumber, lenValue := DecodeTag(buf)

	if tagNumber != bacnet.ApplicationTag_OBJECT_ID {
		return errors.New("invalid object id application tag")
	}

	// Object type & instance
	objectType, objectInstance := DecodeObjectId(buf)

	if objectType != bacnet.OBJECT_DEVICE {
		return errors.New("object type isn't a device")
	}

	dest.DeviceID = objectInstance

	// Max APDU
	tagNumber, lenValue = DecodeTag(buf)

	if tagNumber != bacnet.ApplicationTag_UNSIGNED_INT {
		return errors.New("invalid max apdu application tag")
	}

	dest.MaxAPDU = DecodeUnsigned(buf, lenValue)

	// Segmentation
	tagNumber, lenValue = DecodeTag(buf)

	if tagNumber != types.ApplicationTag_ENUMERATED {
		return errors.New("invalid segmentation application tag")
	}

	segmentation := DecodeUnsigned(buf, lenValue)

	if segmentation >= types.MAX_BACNET_SEGMENTATION {
		return errors.New("invalid segmentation value")
	}

	dest.Segmentation = uint8(segmentation)

	// Vendor ID
	tagNumber, lenValue = DecodeTag(buf)

	if tagNumber != types.ApplicationTag_UNSIGNED_INT {
		return errors.New("invalid vendor id application tag")
	}

	vendorId := DecodeUnsigned(buf, lenValue)

	if vendorId > 0xFFFF {
		return errors.New("invalid vendor id")
	}

	dest.VendorID = uint16(vendorId)

	return nil
}
