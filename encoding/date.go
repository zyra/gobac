package encoding

import "github.com/zyra/gobac/types"

func (buf *Buffer) EncodeDate(date *types.Date) error {
	return buf.AppendBytes([]byte{
		uint8(date.Year - 1900),
		date.Month,
		date.Day,
		date.Weekday,
	})
}

func (buf *Buffer) DecodeDate() *types.Date {
	b := buf.Next(4)

	return &types.Date{
		Year:    uint16(b[0]) + 1900,
		Month:   b[1],
		Day:     b[2],
		Weekday: b[3],
	}
}
