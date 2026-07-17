package bacnet

import (
	"context"
	"testing"
	"time"

	"github.com/zyra/gobac/v2/bacnet/types"
)

func TestBacnetDateTimeSundayConvertsToWeekdaySeven(t *testing.T) {
	// 2026-07-19 is a Sunday.
	date, clock := bacnetDateTime(time.Date(2026, time.July, 19, 14, 30, 15, 250000000, time.UTC))

	wantDate := types.Date{Year: 2026, Month: 7, Day: 19, Weekday: types.WeekdaySunday}
	if date != wantDate {
		t.Fatalf("date = %+v, want %+v", date, wantDate)
	}
	if date.Weekday != 7 {
		t.Fatalf("weekday = %d, want 7", date.Weekday)
	}

	wantTime := types.Time{Hour: 14, Min: 30, Sec: 15, Hundredths: 25}
	if clock != wantTime {
		t.Fatalf("time = %+v, want %+v", clock, wantTime)
	}
}

func TestBacnetDateTimeFridayConvertsToWeekdayFive(t *testing.T) {
	// 2026-07-17 is a Friday.
	date, _ := bacnetDateTime(time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC))

	if date.Weekday != types.WeekdayFriday {
		t.Fatalf("weekday = %d, want %d", date.Weekday, types.WeekdayFriday)
	}
}

func TestSendTimeSyncRequiresListeningServer(t *testing.T) {
	s := newLifecycleTestServer()

	err := s.SendTimeSync(context.Background(), time.Date(2026, time.July, 17, 14, 30, 15, 250000000, time.UTC))
	if err == nil || err.Error() != "server is not listening" {
		t.Fatalf("SendTimeSync error = %v, want %q", err, "server is not listening")
	}
}

func TestSendUTCTimeSyncRequiresListeningServer(t *testing.T) {
	s := newLifecycleTestServer()

	err := s.SendUTCTimeSync(context.Background(), time.Date(2026, time.July, 17, 14, 30, 15, 250000000, time.UTC))
	if err == nil || err.Error() != "server is not listening" {
		t.Fatalf("SendUTCTimeSync error = %v, want %q", err, "server is not listening")
	}
}
