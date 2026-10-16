package timedated

import (
	"bufio"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

type fake struct {
	mu       sync.Mutex
	zone     string
	localRTC bool
	ntp      bool
	clock    time.Time
	synced   int
}

func (f *fake) Zone() string { f.mu.Lock(); defer f.mu.Unlock(); return f.zone }
func (f *fake) SetZone(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.zone = name
	return nil
}
func (f *fake) ListZones() ([]string, error) { return []string{"Europe/Rome", "UTC"}, nil }
func (f *fake) LocalRTC() bool               { f.mu.Lock(); defer f.mu.Unlock(); return f.localRTC }
func (f *fake) SetLocalRTC(l bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.localRTC = l
	return nil
}
func (f *fake) CanNTP() bool               { return true }
func (f *fake) NTP() bool                  { f.mu.Lock(); defer f.mu.Unlock(); return f.ntp }
func (f *fake) SetNTP(e bool) error        { f.mu.Lock(); defer f.mu.Unlock(); f.ntp = e; return nil }
func (f *fake) Synchronized() bool         { f.mu.Lock(); defer f.mu.Unlock(); return f.ntp }
func (f *fake) SetClock(t time.Time) error { f.mu.Lock(); defer f.mu.Unlock(); f.clock = t; return nil }
func (f *fake) SyncRTC(local bool) error   { f.mu.Lock(); defer f.mu.Unlock(); f.synced++; return nil }
func (f *fake) RTC() (time.Time, error)    { return time.Unix(1_700_000_000, 0).UTC(), nil }

func privateBus(t *testing.T) string {
	cmd := exec.Command("dbus-daemon", "--session", "--nofork", "--print-address")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Skip("dbus-daemon not available")
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(line)
}

func TestService(t *testing.T) {
	addr := privateBus(t)
	server, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	f := &fake{zone: "UTC"}
	svc := New(server, f)
	var allow atomic.Bool
	svc.isAdminHook = func(uint32) bool { return allow.Load() }
	if err := svc.Export(); err != nil {
		t.Fatal(err)
	}
	if _, err := server.RequestName(BusName, dbus.NameFlagDoNotQueue); err != nil {
		t.Fatal(err)
	}
	client, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	obj := client.Object(BusName, ObjectPath)

	zone, err := obj.GetProperty(Iface + ".Timezone")
	if err != nil || zone.Value().(string) != "UTC" {
		t.Fatalf("Timezone = %v %v", zone, err)
	}
	if err := obj.Call(Iface+".SetTimezone", 0, "Europe/Rome", false).Err; err == nil {
		t.Fatal("a non-admin caller without polkit must be refused")
	}
	allow.Store(true)
	if err := obj.Call(Iface+".SetTimezone", 0, "Europe/Rome", false).Err; err != nil {
		t.Fatal(err)
	}
	if f.Zone() != "Europe/Rome" {
		t.Fatalf("zone = %q", f.Zone())
	}
	if err := obj.Call(Iface+".SetNTP", 0, true, false).Err; err != nil {
		t.Fatal(err)
	}
	err = obj.Call(Iface+".SetTime", 0, int64(1_800_000_000_000_000), false, false).Err
	if dbusErr, ok := err.(dbus.Error); !ok || dbusErr.Name != "org.freedesktop.timedate1.AutomaticTimeSyncEnabled" {
		t.Fatalf("SetTime with NTP on: %v", err)
	}
	if err := obj.Call(Iface+".SetNTP", 0, false, false).Err; err != nil {
		t.Fatal(err)
	}
	if err := obj.Call(Iface+".SetTime", 0, int64(1_800_000_000_000_000), false, false).Err; err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	clock, synced := f.clock, f.synced
	f.mu.Unlock()
	if clock.UnixMicro() != 1_800_000_000_000_000 || synced == 0 {
		t.Fatalf("clock %v synced %d", clock, synced)
	}
	var zones []string
	if err := obj.Call(Iface+".ListTimezones", 0).Store(&zones); err != nil || len(zones) != 2 {
		t.Fatalf("ListTimezones %v %v", zones, err)
	}
	if err := obj.Call(Iface+".SetLocalRTC", 0, true, false, false).Err; err != nil || !f.LocalRTC() {
		t.Fatalf("SetLocalRTC %v %v", err, f.LocalRTC())
	}
}
