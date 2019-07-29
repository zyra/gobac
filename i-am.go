package gobac

import (
	"errors"
	"github.com/zyra/bacnet"
	"github.com/zyra/gobac/encoding"
	"github.com/zyra/gobac/types"
)

type IAmServiceRequest []byte

func (r *IAmServiceRequest) Decode(dest *Device) error {
	buf := encoding.NewBuffer(*r)

	tagNumber, lenValue := buf.DecodeTag()

	if tagNumber != bacnet.ApplicationTag_OBJECT_ID {
		return errors.New("invalid object id application tag")
	}

	// Object type & instance
	objectType, objectInstance := encoding.DecodeObjectId(buf.Buffer)

	if objectType != bacnet.OBJECT_DEVICE {
		return errors.New("object type isn't a device")
	}

	dest.Type = objectType
	dest.Instance = objectInstance

	// Max APDU
	tagNumber, lenValue = buf.DecodeTag()

	if tagNumber != bacnet.ApplicationTag_UNSIGNED_INT {
		return errors.New("invalid max apdu application tag")
	}

	dest.MaxAPDU = buf.DecodeUnsigned(lenValue)

	// Segmentation
	tagNumber, lenValue = buf.DecodeTag()

	if tagNumber != types.ApplicationTag_ENUMERATED {
		return errors.New("invalid segmentation application tag")
	}

	segmentation := buf.DecodeUnsigned(lenValue)

	if segmentation >= types.MAX_BACNET_SEGMENTATION {
		return errors.New("invalid segmentation value")
	}

	dest.Segmentation = uint8(segmentation)

	// Vendor ID
	tagNumber, lenValue = buf.DecodeTag()

	if tagNumber != types.ApplicationTag_UNSIGNED_INT {
		return errors.New("invalid vendor id application tag")
	}

	vendorId := buf.DecodeUnsigned(lenValue)

	if vendorId > 0xFFFF {
		return errors.New("invalid vendor id")
	}

	dest.VendorID = uint16(vendorId)

	return nil
}
