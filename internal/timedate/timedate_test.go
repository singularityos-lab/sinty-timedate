package timedate

import (
	"os"
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
		"0":     {false, false},
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

func TestRTCRejectsImpossibleDates(t *testing.T) {
	if err := WriteRTC(time.Date(1960, time.January, 1, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Error("a date before 1970 reached the RTC")
	}
}

func TestNTPWithoutServiceManager(t *testing.T) {
	old := controlSocket
	controlSocket = t.TempDir() + "/none.sock"
	defer func() { controlSocket = old }()
	if HasServiceManager() {
		t.Fatal("service manager found at a missing socket")
	}
	if err := SetNTP(true); err != ErrNoServiceManager {
		t.Fatalf("SetNTP: %v", err)
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

func TestZoneIndirection(t *testing.T) {
	dir := t.TempDir()
	persistent := filepath.Join(dir, "var", "localtime")
	if err := os.MkdirAll(filepath.Dir(persistent), 0o755); err != nil {
		t.Fatal(err)
	}
	etc := filepath.Join(dir, "etc")
	if err := os.MkdirAll(etc, 0o755); err != nil {
		t.Fatal(err)
	}
	oldLocal, oldZone := LocaltimePath, TimezonePath
	defer func() { LocaltimePath, TimezonePath = oldLocal, oldZone }()
	LocaltimePath = filepath.Join(etc, "localtime")
	TimezonePath = filepath.Join(etc, "timezone")
	if err := os.Symlink("../var/localtime", LocaltimePath); err != nil {
		t.Fatal(err)
	}
	if err := SetZone("Europe/Rome"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.Readlink(LocaltimePath); got != "../var/localtime" {
		t.Fatalf("the /etc link was replaced: %q", got)
	}
	if got, _ := os.Readlink(persistent); got != filepath.Join(ZoneinfoDir, "Europe/Rome") {
		t.Fatalf("persistent link = %q", got)
	}
	if z := CurrentZone(); z != "Europe/Rome" {
		t.Fatalf("CurrentZone = %q", z)
	}
	if _, err := os.Stat(TimezonePath); !os.IsNotExist(err) {
		t.Fatal("the zone name must be written next to the persistent link")
	}
}
