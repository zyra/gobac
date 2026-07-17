package store

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestDeviceStoreSnapshotOrder(t *testing.T) {
	s := NewDeviceStore()
	s.Upsert(DeviceRow{Key: DeviceKey{Instance: 70002, IP: "192.0.2.1"}})
	s.Upsert(DeviceRow{Key: DeviceKey{Instance: 70001, IP: "192.0.2.20"}})
	s.Upsert(DeviceRow{Key: DeviceKey{Instance: 70001, IP: "192.0.2.10"}})

	got := s.Snapshot()
	if len(got) != 3 {
		t.Fatalf("Snapshot() len = %d, want 3", len(got))
	}

	wantKeys := []DeviceKey{
		{Instance: 70001, IP: "192.0.2.10"},
		{Instance: 70001, IP: "192.0.2.20"},
		{Instance: 70002, IP: "192.0.2.1"},
	}
	for i, want := range wantKeys {
		if got[i].Key != want {
			t.Errorf("Snapshot()[%d].Key = %+v, want %+v", i, got[i].Key, want)
		}
	}
}

func TestDeviceStoreUpsertMergesAndAdvancesLastSeen(t *testing.T) {
	s := NewDeviceStore()
	t1 := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 7, 16, 12, 0, 5, 0, time.UTC)
	key := DeviceKey{Instance: 1001, IP: "127.0.0.1"}

	s.Now = func() time.Time { return t1 }
	s.Upsert(DeviceRow{Key: key, VendorID: 5})

	s.Now = func() time.Time { return t2 }
	s.Upsert(DeviceRow{Key: key, VendorID: 9})

	if got := s.Len(); got != 1 {
		t.Fatalf("Len() = %d, want 1", got)
	}

	rows := s.Snapshot()
	if len(rows) != 1 {
		t.Fatalf("Snapshot() len = %d, want 1", len(rows))
	}
	if rows[0].VendorID != 9 {
		t.Errorf("VendorID = %d, want 9", rows[0].VendorID)
	}
	if !rows[0].LastSeen.Equal(t2) {
		t.Errorf("LastSeen = %v, want %v", rows[0].LastSeen, t2)
	}
}

func TestDeviceStoreListenerFiresExactlyOncePerMutation(t *testing.T) {
	s := NewDeviceStore()
	var count int
	remove := s.AddListener(func() { count++ })

	s.Upsert(DeviceRow{Key: DeviceKey{Instance: 1, IP: "127.0.0.1"}})
	if count != 1 {
		t.Fatalf("count after Upsert = %d, want 1", count)
	}

	s.Remove(DeviceKey{Instance: 1, IP: "127.0.0.1"})
	if count != 2 {
		t.Fatalf("count after Remove = %d, want 2", count)
	}

	s.Clear()
	if count != 3 {
		t.Fatalf("count after Clear = %d, want 3", count)
	}

	remove()
	s.Upsert(DeviceRow{Key: DeviceKey{Instance: 2, IP: "127.0.0.1"}})
	if count != 3 {
		t.Fatalf("count after remove()+Upsert = %d, want 3 (unchanged)", count)
	}
}

func TestDeviceStoreConcurrentUpserts(t *testing.T) {
	s := NewDeviceStore()
	var wg sync.WaitGroup
	for g := 0; g < 100; g++ {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				key := DeviceKey{Instance: uint32(g), IP: fmt.Sprintf("127.0.0.%d", i)}
				s.Upsert(DeviceRow{Key: key})
			}
		}()
	}
	wg.Wait()

	if got := s.Len(); got != 10000 {
		t.Fatalf("Len() = %d, want 10000", got)
	}
}

func TestDeviceStoreSourceStaysLocalSim(t *testing.T) {
	s := NewDeviceStore()
	key := DeviceKey{Instance: 1001, IP: "127.0.0.1"}

	s.Upsert(DeviceRow{Key: key, Source: "local-sim"})
	s.Upsert(DeviceRow{Key: key, Source: "network"})

	rows := s.Snapshot()
	if len(rows) != 1 {
		t.Fatalf("Snapshot() len = %d, want 1", len(rows))
	}
	if rows[0].Source != "local-sim" {
		t.Errorf("Source = %q, want %q", rows[0].Source, "local-sim")
	}
}

func TestDeviceStoreSnapshotIsolation(t *testing.T) {
	s := NewDeviceStore()
	s.Upsert(DeviceRow{Key: DeviceKey{Instance: 1, IP: "127.0.0.1"}, VendorID: 5})

	rows := s.Snapshot()
	rows[0].VendorID = 999
	rows = append(rows, DeviceRow{Key: DeviceKey{Instance: 2, IP: "127.0.0.1"}})

	again := s.Snapshot()
	if len(again) != 1 {
		t.Fatalf("Snapshot() len = %d, want 1", len(again))
	}
	if again[0].VendorID != 5 {
		t.Errorf("VendorID = %d, want 5 (external mutation must not affect store)", again[0].VendorID)
	}
}
