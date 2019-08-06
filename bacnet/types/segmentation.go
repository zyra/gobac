package types

const (
	SegmentationBoth     Segmentation = 0
	SegmentationTransmit              = 1
	SegmentationReceive               = 2
	SegmentationNone                  = 3
	SegmentationMax                   = 4
)

type Segmentation uint8
