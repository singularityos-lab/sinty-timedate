package timedate

import (
	"path/filepath"
	"testing"
	"time"
)

func TestParseBoolArg(t *testing.T) {
	cases := map[string]struct {
		want bool
		err  bool
	}{
		"true": {true, false}, "TRUE": {true, false}, "yes": {true, false},
		"on": {true, false}, "1": {true, false},
		"false": {false, false}, "no": {false, false}, "off": {false, false},
		"0": {false, false},
		"maybe": {false, true}, "": {false, true},
	}
	for in, c := range cases {
		got, err := ParseBoolArg(in)
		if c.err {
			if err == nil {
				t.Errorf("ParseBoolArg(%q): expected error", in)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("ParseBoolArg(%q) = %v, %v; want %v", in, got, err, c.want)
		}
	}
}

func TestValidZone(t *testing.T) {
	// These exist in every tzdata install used by the target.
	for _, z := range []string{"UTC", "Europe/Rome", "America/New_York", "Etc/GMT-14"} {
		if !ValidZone(z) {
			t.Errorf("ValidZone(%q) = false, want true", z)
		}
	}
	for _, z := range []string{"", "Not/AZone", "../etc/passwd", "/etc/passwd", "Europe/", "posix/UTC"} {
		if ValidZone(z) {
			t.Errorf("ValidZone(%q) = true, want false", z)
		}
	}
}

func TestZoneFromPath(t *testing.T) {
	cases := map[string]string{
		"../usr/share/zoneinfo/Europe/Rome": "Europe/Rome",
		"/usr/share/zoneinfo/UTC":           "UTC",
		"/etc/localtime":                    "",
	}
	for in, want := range cases {
		if got := zoneFromPath(in); got != want {
			t.Errorf("zoneFromPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRTCRoundTrip(t *testing.T) {
	want := time.Date(2026, time.July, 12, 22, 5, 30, 0, time.UTC)
	got := rtcToTime(ptr(timeToRTC(want)))
	if !got.Equal(want) {
		t.Errorf("RTC round-trip = %v, want %v", got, want)
	}
}

func TestRTCWallFor(t *testing.T) {
	instant := time.Date(2026, time.July, 12, 20, 0, 0, 0, time.UTC)
	if got := rtcWallFor(instant, false); got.Hour() != 20 {
		t.Errorf("rtcWallFor UTC hour = %d, want 20", got.Hour())
	}
	// With local-RTC the stored fields are the local wall clock, whatever the
	// host's zone; we only assert the carrier is a valid time.
	if got := rtcWallFor(instant, true); got.IsZero() {
		t.Error("rtcWallFor local returned zero time")
	}
}

func TestConfigRoundTrip(t *testing.T) {
	old := ConfigPath
	ConfigPath = filepath.Join(t.TempDir(), "sinty-timedate.conf")
	defer func() { ConfigPath = old }()

	// Missing file -> zero value.
	if c, err := LoadConfig(); err != nil || c.LocalRTC {
		t.Fatalf("LoadConfig(missing) = %+v, %v; want {false}, nil", c, err)
	}
	if err := (Config{LocalRTC: true}).Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	c, err := LoadConfig()
	if err != nil || !c.LocalRTC {
		t.Fatalf("LoadConfig = %+v, %v; want LocalRTC=true", c, err)
	}
}

func ptr[T any](v T) *T { return &v }
