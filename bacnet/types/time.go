package types

import (
	"fmt"
	"github.com/kataras/iris/core/errors"
	"time"
)

type Time struct {
	Hour       uint8
	Min        uint8
	Sec        uint8
	Hundredths uint8
}

func (t Time) String() string {
	return fmt.Sprintf("%02d:%02d:%02d.%02d", t.Hour, t.Min, t.Sec, t.Hundredths)
}

func (t Time) GoTime() (time.Time, error) {
	return time.Parse("15:04:05.000", t.String())
}

func (t Time) MarshalBinary() ([]byte, error) {
	return []byte{
		t.Hour,
		t.Min,
		t.Sec,
		t.Hundredths,
	}, nil
}

func (t *Time) UnmarshalBinary(data []byte) error {
	if len(data) < 4 {
		return errors.New("time binary must have 4 octets")
	}

	t.Hour = data[0]
	t.Min = data[1]
	t.Sec = data[2]
	t.Hundredths = data[3]
	return nil
}
