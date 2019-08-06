package types

import (
	"fmt"
	"time"
)

type DateTime struct {
	Date Date
	Time Time
}

func (dt DateTime) String() string {
	return fmt.Sprintf("%sT%s", dt.Date.String(), dt.Time.String())
}

func (dt DateTime) GoTime() (time.Time, error) {
	return time.Parse(time.RFC3339, dt.String())
}
