package encoding

import "github.com/zyra/gobac/types"

func (buf *Buffer) EncodeTime(time *types.Time) error {
	return buf.AppendBytes([]byte{
		time.Hour,
		time.Min,
		time.Sec,
		time.Hundredths,
	})
}

func (buf *Buffer) DecodeTime() *types.Time {
	b := buf.Next(4)
	return &types.Time{
		Hour:       b[0],
		Min:        b[1],
		Sec:        b[2],
		Hundredths: b[3],
	}
}
