package store

import (
	"testing"

	"github.com/zyra/gobac/gui/internal/session"
)

func TestObjectCacheObjectsRoundTrip(t *testing.T) {
	c := NewObjectCache()
	dev := DeviceKey{Instance: 1001, IP: "127.0.0.1"}
	other := DeviceKey{Instance: 1002, IP: "127.0.0.1"}

	entries := []ObjectEntry{
		{Type: 2, Instance: 1, Name: "Setpoint"},
		{Type: 5, Instance: 3, Name: "Fan"},
	}
	c.SetObjects(dev, entries)

	got := c.Objects(dev)
	if len(got) != 2 {
		t.Fatalf("Objects(dev) len = %d, want 2", len(got))
	}
	if got[0] != entries[0] || got[1] != entries[1] {
		t.Errorf("Objects(dev) = %+v, want %+v", got, entries)
	}

	if got := c.Objects(other); got != nil {
		t.Errorf("Objects(other) = %+v, want nil (no entry set for that device)", got)
	}
}

func TestObjectCacheObjectsIsolation(t *testing.T) {
	c := NewObjectCache()
	dev := DeviceKey{Instance: 1001, IP: "127.0.0.1"}

	entries := []ObjectEntry{{Type: 2, Instance: 1, Name: "Setpoint"}}
	c.SetObjects(dev, entries)

	got := c.Objects(dev)
	got[0].Name = "tampered"
	got = append(got, ObjectEntry{Type: 3, Instance: 9, Name: "Injected"})

	again := c.Objects(dev)
	if len(again) != 1 {
		t.Fatalf("Objects(dev) len = %d, want 1", len(again))
	}
	if again[0].Name != "Setpoint" {
		t.Errorf("Name = %q, want %q (external mutation must not affect cache)", again[0].Name, "Setpoint")
	}

	// Mutating the input slice after SetObjects must not affect the cache
	// either.
	entries[0].Name = "also-tampered"
	again = c.Objects(dev)
	if again[0].Name != "Setpoint" {
		t.Errorf("Name = %q, want %q (mutating the input slice after SetObjects must not affect cache)", again[0].Name, "Setpoint")
	}
}

func TestObjectCachePropertiesRoundTrip(t *testing.T) {
	c := NewObjectCache()
	dev := DeviceKey{Instance: 1001, IP: "127.0.0.1"}
	obj := session.ObjectRef{Type: 2, Instance: 1}
	otherObj := session.ObjectRef{Type: 2, Instance: 2}

	props := []PropertyEntry{
		{ID: 85, Values: []session.Value{{Tag: 4, Value: float32(42.5)}}},
		{ID: 999999, Err: "unknown-property"},
	}
	c.SetProperties(dev, obj, props)

	got := c.Properties(dev, obj)
	if len(got) != 2 {
		t.Fatalf("Properties(dev, obj) len = %d, want 2", len(got))
	}
	if got[0].ID != 85 || len(got[0].Values) != 1 || got[0].Values[0].Value != float32(42.5) {
		t.Errorf("Properties(dev, obj)[0] = %+v, want Present_Value 42.5", got[0])
	}
	if got[1].ID != 999999 || got[1].Err != "unknown-property" {
		t.Errorf("Properties(dev, obj)[1] = %+v, want Err %q", got[1], "unknown-property")
	}

	if got := c.Properties(dev, otherObj); got != nil {
		t.Errorf("Properties(dev, otherObj) = %+v, want nil (no entry set for that object)", got)
	}
}

func TestObjectCacheListenerFiresExactlyOncePerMutation(t *testing.T) {
	c := NewObjectCache()
	dev := DeviceKey{Instance: 1001, IP: "127.0.0.1"}
	obj := session.ObjectRef{Type: 2, Instance: 1}

	var count int
	remove := c.AddListener(func() { count++ })

	c.SetObjects(dev, []ObjectEntry{{Type: 2, Instance: 1, Name: "Setpoint"}})
	if count != 1 {
		t.Fatalf("count after SetObjects = %d, want 1", count)
	}

	c.SetProperties(dev, obj, []PropertyEntry{{ID: 85}})
	if count != 2 {
		t.Fatalf("count after SetProperties = %d, want 2", count)
	}

	remove()
	c.SetObjects(dev, nil)
	if count != 2 {
		t.Fatalf("count after remove()+SetObjects = %d, want 2 (unchanged)", count)
	}
}
