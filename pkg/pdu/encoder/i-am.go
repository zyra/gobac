package encoder

import (
	"bytes"
	"github.com/kataras/iris/core/errors"
	"github.com/zyra/bacnet"
	"github.com/zyra/bacnet-2/pkg/object"
	_type "github.com/zyra/bacnet-2/pkg/type"
	"github.com/zyra/bacnet-2/pkg/util"
)

type IAmServiceRequest []byte

func (r *IAmServiceRequest) Decode(dest *object.Device) error {
	buf := bytes.NewBuffer(*r)
	tagNumber, lenValue := util.DecodeTag(buf)

	if tagNumber != bacnet.ApplicationTag_OBJECT_ID {
		return errors.New("invalid object id application tag")
	}

	// Object type & instance
	objectType, objectInstance := util.DecodeObjectId(buf)

	if objectType != bacnet.OBJECT_DEVICE {
		return errors.New("object type isn't a device")
	}

	dest.DeviceID = objectInstance

	// Max APDU
	tagNumber, lenValue = util.DecodeTag(buf)

	if tagNumber != bacnet.ApplicationTag_UNSIGNED_INT {
		return errors.New("invalid max apdu application tag")
	}

	dest.MaxAPDU = util.DecodeUnsigned(buf, lenValue)

	// Segmentation
	tagNumber, lenValue = util.DecodeTag(buf)

	if tagNumber != _type.ApplicationTag_ENUMERATED {
		return errors.New("invalid segmentation application tag")
	}

	segmentation := util.DecodeUnsigned(buf, lenValue)

	if segmentation >= _type.MAX_BACNET_SEGMENTATION {
		return errors.New("invalid segmentation value")
	}

	dest.Segmentation = uint8(segmentation)

	// Vendor ID
	tagNumber, lenValue = util.DecodeTag(buf)

	if tagNumber != _type.ApplicationTag_UNSIGNED_INT {
		return errors.New("invalid vendor id application tag")
	}

	vendorId := util.DecodeUnsigned(buf, lenValue)

	if vendorId > 0xFFFF {
		return errors.New("invalid vendor id")
	}

	dest.VendorID = uint16(vendorId)

	return nil
}
