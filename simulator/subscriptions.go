package simulator

import (
	"sort"
	"sync"
	"time"
)

type SubscriptionKey struct {
	Subscriber string
	ProcessID  uint32
	Object     ObjectID
}

type Subscription struct {
	Key       SubscriptionKey
	Confirmed bool
	Lifetime  time.Duration
	ExpiresAt time.Time
	LastValue []Value
}

type SubscriptionRegistry struct {
	clock Clock
	mu    sync.Mutex
	items map[SubscriptionKey]*Subscription
}

func NewSubscriptionRegistry(clock Clock) *SubscriptionRegistry {
	if clock == nil {
		clock = RealClock{}
	}
	return &SubscriptionRegistry{clock: clock, items: make(map[SubscriptionKey]*Subscription)}
}

func (r *SubscriptionRegistry) Subscribe(subscription Subscription) {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := subscription
	copy.LastValue = cloneValues(subscription.LastValue)
	if copy.Lifetime > 0 {
		copy.ExpiresAt = r.clock.Now().Add(copy.Lifetime)
	} else {
		copy.ExpiresAt = time.Time{}
	}
	r.items[copy.Key] = &copy
}

func (r *SubscriptionRegistry) Cancel(key SubscriptionKey) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[key]; !exists {
		return false
	}
	delete(r.items, key)
	return true
}

func (r *SubscriptionRegistry) Active() []Subscription {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeExpiredLocked()
	result := make([]Subscription, 0, len(r.items))
	for _, item := range r.items {
		copy := *item
		copy.LastValue = cloneValues(item.LastValue)
		result = append(result, copy)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Key.Subscriber != result[j].Key.Subscriber {
			return result[i].Key.Subscriber < result[j].Key.Subscriber
		}
		if result[i].Key.ProcessID != result[j].Key.ProcessID {
			return result[i].Key.ProcessID < result[j].Key.ProcessID
		}
		if result[i].Key.Object.Type != result[j].Key.Object.Type {
			return result[i].Key.Object.Type < result[j].Key.Object.Type
		}
		return result[i].Key.Object.Instance < result[j].Key.Object.Instance
	})
	return result
}

func (r *SubscriptionRegistry) UpdateLastValue(key SubscriptionKey, values []Value) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeExpiredLocked()
	item := r.items[key]
	if item == nil {
		return false
	}
	item.LastValue = cloneValues(values)
	return true
}

func (r *SubscriptionRegistry) removeExpiredLocked() {
	now := r.clock.Now()
	for key, item := range r.items {
		if !item.ExpiresAt.IsZero() && !now.Before(item.ExpiresAt) {
			delete(r.items, key)
		}
	}
}
