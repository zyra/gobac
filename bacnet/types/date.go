package types

import (
	"fmt"
	"time"
)

type Date struct {
	Year    uint16
	Month   uint8
	Day     uint8
	Weekday Weekday
}

func (d Date) MarshalBinary() ([]byte, error) {
	return []byte{
		uint8(d.Year - 1900),
		d.Month,
		d.Day,
		byte(d.Weekday),
	}, nil
}

func (d *Date) UnmarshalBinary(b []byte) error {
	d.Year = uint16(b[0]) + 1900
	d.Month = b[1]
	d.Day = b[2]
	d.Weekday = Weekday(b[3])
	return nil
}

func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Weekday)
}

func (d Date) GoTime() (time.Time, error) {
	return time.Parse("2006-01-02", d.String())
}
