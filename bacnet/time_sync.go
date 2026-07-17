package bacnet

import (
	"context"
	"time"

	"github.com/zyra/gobac/v2/bacnet/pdu"
	"github.com/zyra/gobac/v2/bacnet/types"
)

// SendTimeSync broadcasts a (local) TimeSynchronization with the given wall
// time (ASHRAE 135 §16.10). It is fire-and-forget: TimeSynchronization has
// no reply, so no handler is registered and this returns as soon as the
// broadcast is sent.
func (s *Server) SendTimeSync(ctx context.Context, t time.Time) error {
	return s.sendTimeSync(types.UnconfirmedServiceTimeSynchronization, t)
}

// SendUTCTimeSync broadcasts a UTCTimeSynchronization (ASHRAE 135 §16.11); t
// is converted with t.UTC() before encoding. It is fire-and-forget: no reply
// is expected and no handler is registered.
func (s *Server) SendUTCTimeSync(ctx context.Context, t time.Time) error {
	return s.sendTimeSync(types.UnconfirmedServiceUtcTimeSynchronization, t.UTC())
}

func (s *Server) sendTimeSync(choice types.UnconfirmedService, t time.Time) error {
	date, clock := bacnetDateTime(t)

	req := NewRequest()
	defer req.Release()

	req.SetUnconfirmedService(choice, &pdu.TimeSyncPdu{Date: date, Time: clock})

	data, err := req.MarshalBinary()
	if err != nil {
		return err
	}

	return s.Send(data, s.GetBroadcastAddr())
}

// bacnetDateTime converts a Go time.Time into the BACnet Date/Time pair used
// by TimeSynchronization: year, month, day, and BACnet weekday (Monday=1 ..
// Sunday=7, converting Go's Sunday=0 time.Weekday), plus hour, minute,
// second, and hundredths (t.Nanosecond() / 1e7).
func bacnetDateTime(t time.Time) (types.Date, types.Time) {
	weekday := t.Weekday()
	bacnetWeekday := types.Weekday(weekday)
	if weekday == time.Sunday {
		bacnetWeekday = types.WeekdaySunday
	}

	date := types.Date{
		Year:    uint16(t.Year()),
		Month:   uint8(t.Month()),
		Day:     uint8(t.Day()),
		Weekday: bacnetWeekday,
	}
	clock := types.Time{
		Hour:       uint8(t.Hour()),
		Min:        uint8(t.Minute()),
		Sec:        uint8(t.Second()),
		Hundredths: uint8(t.Nanosecond() / 1e7),
	}
	return date, clock
}
