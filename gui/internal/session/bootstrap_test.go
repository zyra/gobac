package session

import (
	"errors"
	"testing"
)

// fakeStarter is a minimal Starter fake recording Start/Stop calls without
// touching real sockets.
type fakeStarter struct {
	startCfg   Config
	startErr   error
	startCalls int
	stopCalls  int
}

var _ Starter = (*fakeStarter)(nil)

func (f *fakeStarter) Start(cfg Config) error {
	f.startCalls++
	f.startCfg = cfg
	return f.startErr
}

func (f *fakeStarter) Stop() error {
	f.stopCalls++
	return nil
}

func TestConfigFromSettingsMapsInterfaceAndPort(t *testing.T) {
	got := ConfigFromSettings("eno0", 47808)
	want := Config{Interface: "eno0", Port: 47808, LocalPort: 47808}
	if got != want {
		t.Errorf("ConfigFromSettings(%q, %d) = %+v, want %+v", "eno0", 47808, got, want)
	}
}

func TestStartFromSettingsPassesMappedConfig(t *testing.T) {
	f := &fakeStarter{}

	if err := StartFromSettings(f, "eno0", 47808); err != nil {
		t.Fatalf("StartFromSettings: %v", err)
	}

	want := Config{Interface: "eno0", Port: 47808, LocalPort: 47808}
	if f.startCfg != want {
		t.Errorf("Start called with %+v, want %+v", f.startCfg, want)
	}
	if f.startCalls != 1 {
		t.Errorf("Start called %d times, want 1", f.startCalls)
	}
}

func TestStartFromSettingsPropagatesError(t *testing.T) {
	wantErr := errors.New("bind failed")
	f := &fakeStarter{startErr: wantErr}

	err := StartFromSettings(f, "eno0", 47808)

	if !errors.Is(err, wantErr) {
		t.Errorf("StartFromSettings error = %v, want %v", err, wantErr)
	}
}

func TestShutdownStopsStarter(t *testing.T) {
	f := &fakeStarter{}

	if err := Shutdown(f); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if f.stopCalls != 1 {
		t.Errorf("Stop called %d times, want 1", f.stopCalls)
	}
}
