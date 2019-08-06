package types

import "time"

type Weekday uint8

const (
	WeekdayMonday    Weekday = 1
	WeekdayTuesday           = 2
	WeekdayWednesday         = 3
	WeekdayThursday          = 4
	WeekdayFriday            = 5
	WeekdaySaturday          = 6
	WeekdaySunday            = 7
)

func (d Weekday) String() string {
	day := int(d)

	if day == 7 {
		day = 0
	}
	return time.Weekday(day).String()
}
