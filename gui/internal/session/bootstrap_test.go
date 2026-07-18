package session

import (
	"errors"
	"testing"

	"github.com/zyra/gobac/gui/internal/netpick"
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

// withResolveAutomatic temporarily replaces the resolveAutomatic seam,
// restoring the original on cleanup.
func withResolveAutomatic(t *testing.T, fn func() (netpick.Candidate, bool)) {
	t.Helper()
	orig := resolveAutomatic
	resolveAutomatic = fn
	t.Cleanup(func() { resolveAutomatic = orig })
}

func TestStartFromSettingsResolvesEmptyInterfaceToAutomaticPick(t *testing.T) {
	withResolveAutomatic(t, func() (netpick.Candidate, bool) {
		return netpick.Candidate{Name: "fake0"}, true
	})
	f := &fakeStarter{}

	if err := StartFromSettings(f, "", 47808); err != nil {
		t.Fatalf("StartFromSettings: %v", err)
	}

	want := Config{Interface: "fake0", Port: 47808, LocalPort: 47808}
	if f.startCfg != want {
		t.Errorf("Start called with %+v, want %+v", f.startCfg, want)
	}
}

func TestStartFromSettingsErrorsWhenNoCandidates(t *testing.T) {
	withResolveAutomatic(t, func() (netpick.Candidate, bool) {
		return netpick.Candidate{}, false
	})
	f := &fakeStarter{}

	err := StartFromSettings(f, "", 47808)
	if err == nil || err.Error() != "no usable network found" {
		t.Fatalf("StartFromSettings error = %v, want %q", err, "no usable network found")
	}
	if f.startCalls != 0 {
		t.Errorf("Start called %d times, want 0", f.startCalls)
	}
}

func TestStartFromSettingsPassesExplicitInterfaceUnchanged(t *testing.T) {
	withResolveAutomatic(t, func() (netpick.Candidate, bool) {
		t.Fatal("resolveAutomatic called for an explicit interface")
		return netpick.Candidate{}, false
	})
	f := &fakeStarter{}

	if err := StartFromSettings(f, "eno0", 47808); err != nil {
		t.Fatalf("StartFromSettings: %v", err)
	}

	want := Config{Interface: "eno0", Port: 47808, LocalPort: 47808}
	if f.startCfg != want {
		t.Errorf("Start called with %+v, want %+v", f.startCfg, want)
	}
}

func TestStartFromSettingsMapsAllNetworksSentinelToWildcard(t *testing.T) {
	withResolveAutomatic(t, func() (netpick.Candidate, bool) {
		t.Fatal("resolveAutomatic called for the all-networks sentinel")
		return netpick.Candidate{}, false
	})
	f := &fakeStarter{}

	if err := StartFromSettings(f, AllNetworksSentinel, 47808); err != nil {
		t.Fatalf("StartFromSettings: %v", err)
	}

	want := Config{Interface: "", Port: 47808, LocalPort: 47808}
	if f.startCfg != want {
		t.Errorf("Start called with %+v, want %+v", f.startCfg, want)
	}
	if f.startCalls != 1 {
		t.Errorf("Start called %d times, want 1", f.startCalls)
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
