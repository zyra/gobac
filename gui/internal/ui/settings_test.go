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
