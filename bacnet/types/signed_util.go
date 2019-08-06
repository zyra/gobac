package types

import "math"

func GetIntLen(val int) int {
	if val <= math.MaxInt8 && val >= math.MinInt16 {
		return 1
	}

	if val <= math.MaxInt16 && val >= math.MinInt16 {
		return 2
	}

	if val <= 8388607 && val >= -8388607 {
		return 3
	}

	return 4
}

func GetUintLen(val uint) int {
	if val <= math.MaxUint8 {
		return 1
	} else if val <= math.MaxUint16 {
		return 2
	} else if val <= 1<<24-1 {
		return 3
	} else {
		return 4
	}
}
