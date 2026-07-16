package simulator

import (
	"testing"
	"time"
)

func TestSubscriptionLifetimeAndRenewal(t *testing.T) {
	clock := NewManualClock(time.Unix(1000, 0))
	registry := NewSubscriptionRegistry(clock)
	key := SubscriptionKey{Subscriber: "127.0.0.1:47809", ProcessID: 70000, Object: ObjectID{Type: 2, Instance: 1}}

	registry.Subscribe(Subscription{Key: key, Lifetime: time.Minute})
	clock.Advance(30 * time.Second)
	registry.Subscribe(Subscription{Key: key, Lifetime: time.Minute})
	clock.Advance(31 * time.Second)
	if got := len(registry.Active()); got != 1 {
		t.Fatalf("active subscriptions = %d, want 1 after renewal", got)
	}

	clock.Advance(30 * time.Second)
	if got := len(registry.Active()); got != 0 {
		t.Fatalf("active subscriptions = %d, want 0 after expiry", got)
	}
}

func TestSubscriptionCancellation(t *testing.T) {
	registry := NewSubscriptionRegistry(NewManualClock(time.Unix(1000, 0)))
	key := SubscriptionKey{Subscriber: "127.0.0.1:47809", ProcessID: 1, Object: ObjectID{Type: 2, Instance: 1}}
	registry.Subscribe(Subscription{Key: key})
	if !registry.Cancel(key) {
		t.Fatal("expected subscription cancellation")
	}
	if registry.Cancel(key) {
		t.Fatal("second cancellation should report no subscription")
	}
}
