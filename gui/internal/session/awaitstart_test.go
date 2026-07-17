package session

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestAwaitStartSurfacesQueuedError covers the failure this guards against:
// bacnet.Server.Listen closes its Start() channel even when ListenContext
// failed (see markStarted in bacnet/server.go), so treating a closed start
// channel as unconditional success hid startup failures (e.g. a listener
// that couldn't bind its socket) behind a later, confusing "server is not
// listening" error from the first Send call. awaitStart must instead report
// the queued startup error.
func TestAwaitStartSurfacesQueuedError(t *testing.T) {
	start := make(chan struct{})
	errs := make(chan error, 1)

	startErr := errors.New("bind: address already in use")
	errs <- startErr
	close(start)

	err := awaitStart(start, errs, time.After(time.Second))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), startErr.Error()) {
		t.Fatalf("expected error to mention %q, got: %v", startErr.Error(), err)
	}
}

// TestAwaitStartSucceedsWithoutQueuedError covers the ordinary successful
// path: start fires and nothing was queued onto errs.
func TestAwaitStartSucceedsWithoutQueuedError(t *testing.T) {
	start := make(chan struct{})
	errs := make(chan error, 1)
	close(start)

	if err := awaitStart(start, errs, time.After(time.Second)); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// TestAwaitStartTimesOut covers a start channel that never fires.
func TestAwaitStartTimesOut(t *testing.T) {
	start := make(chan struct{})
	errs := make(chan error, 1)
	timeout := make(chan time.Time, 1)
	timeout <- time.Now()

	err := awaitStart(start, errs, timeout)
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "did not start") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

// TestAwaitStartToleratesNilErrsChannel covers callers that pass a nil
// Errors() channel (e.g. ReceiveErrors disabled): the non-blocking read must
// fall through to the default case rather than block forever.
func TestAwaitStartToleratesNilErrsChannel(t *testing.T) {
	start := make(chan struct{})
	close(start)

	if err := awaitStart(start, nil, time.After(time.Second)); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
