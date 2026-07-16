package main

import "testing"

func TestNewAppUsesBuildVersion(t *testing.T) {
	original := version
	version = "2.0.0-test"
	defer func() { version = original }()

	if got := newApp().Version; got != version {
		t.Fatalf("version = %q, want %q", got, version)
	}
}
