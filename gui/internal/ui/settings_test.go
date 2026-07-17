package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestValidatePortRejectsInvalidValues(t *testing.T) {
	cases := []string{"0", "70000", "abc"}
	for _, in := range cases {
		if err := validatePort(in); err == nil {
			t.Errorf("validatePort(%q) = nil, want error", in)
		}
	}
}

func TestValidatePortAcceptsInRangeValue(t *testing.T) {
	if err := validatePort("47808"); err != nil {
		t.Errorf("validatePort(%q) = %v, want nil", "47808", err)
	}
}

func TestLoadSettingsDefaultsPortWhenUnset(t *testing.T) {
	a := test.NewApp()

	got := LoadSettings(a)

	if got.Port != DefaultPort {
		t.Errorf("LoadSettings(a).Port = %d, want %d", got.Port, DefaultPort)
	}
	if got.Interface != "" {
		t.Errorf("LoadSettings(a).Interface = %q, want empty", got.Interface)
	}
}

func TestTrySaveSettingsRejectsOutOfRangePort(t *testing.T) {
	a := test.NewApp()
	SaveSettings(a, Settings{Interface: "eno0", Port: DefaultPort})

	if err := trySaveSettings(a, "eth9", "70000"); err == nil {
		t.Fatal("trySaveSettings with out-of-range port = nil error, want error")
	}

	got := LoadSettings(a)
	if got.Interface != "eno0" || got.Port != DefaultPort {
		t.Errorf("LoadSettings(a) = %+v, want unchanged Interface %q Port %d", got, "eno0", DefaultPort)
	}
}

func TestTrySaveSettingsPersistsValidPort(t *testing.T) {
	a := test.NewApp()

	if err := trySaveSettings(a, "eno0", "47809"); err != nil {
		t.Fatalf("trySaveSettings: %v", err)
	}

	got := LoadSettings(a)
	if got.Interface != "eno0" || got.Port != 47809 {
		t.Errorf("LoadSettings(a) = %+v, want Interface %q Port %d", got, "eno0", 47809)
	}
}
