package timedated

import (
	"errors"
	"os/user"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"

	"github.com/singularityos-lab/sinty-timedate/internal/timedate"
)

const (
	BusName    = "org.freedesktop.timedate1"
	ObjectPath = dbus.ObjectPath("/org/freedesktop/timedate1")
	Iface      = "org.freedesktop.timedate1"
	propsIface = "org.freedesktop.DBus.Properties"

	actionTime     = "org.freedesktop.timedate1.set-time"
	actionTimezone = "org.freedesktop.timedate1.set-timezone"
	actionLocalRTC = "org.freedesktop.timedate1.set-local-rtc"
	actionNTP      = "org.freedesktop.timedate1.set-ntp"
	polkitName     = "org.freedesktop.PolicyKit1"
)

type Backend interface {
	Zone() string
	SetZone(name string) error
	ListZones() ([]string, error)
	LocalRTC() bool
	SetLocalRTC(local bool) error
	CanNTP() bool
	NTP() bool
	SetNTP(enable bool) error
	Synchronized() bool
	SetClock(t time.Time) error
	SyncRTC(local bool) error
	RTC() (time.Time, error)
}

type System struct{}

func (System) Zone() string                 { return timedate.CurrentZone() }
func (System) SetZone(name string) error    { return timedate.SetZone(name) }
func (System) ListZones() ([]string, error) { return timedate.ListZones() }
func (System) CanNTP() bool                 { return timedate.HasServiceManager() }
func (System) NTP() bool                    { return timedate.IsNTPActive() }
func (System) SetNTP(enable bool) error     { return timedate.SetNTP(enable) }
func (System) Synchronized() bool           { return timedate.IsSynchronized() }
func (System) SetClock(t time.Time) error   { return timedate.SetSystemClock(t) }
func (System) SyncRTC(local bool) error     { return timedate.SyncRTCFromSystem(local) }
func (System) RTC() (time.Time, error)      { return timedate.ReadRTC() }

func (System) LocalRTC() bool {
	c, err := timedate.LoadConfig()
	return err == nil && c.LocalRTC
}

func (System) SetLocalRTC(local bool) error {
	c, err := timedate.LoadConfig()
	if err != nil {
		return err
	}
	c.LocalRTC = local
	return c.Save()
}

type Service struct {
	conn        *dbus.Conn
	backend     Backend
	mu          sync.Mutex
	adminGroups []string
	isAdminHook func(uint32) bool
}

func New(conn *dbus.Conn, backend Backend) *Service {
	return &Service{conn: conn, backend: backend, adminGroups: []string{"wheel", "sudo", "admin"}}
}

var introspection = introspect.Interface{
	Name: Iface,
	Methods: []introspect.Method{
		{Name: "SetTime", Args: []introspect.Arg{{Name: "usec_utc", Type: "x", Direction: "in"}, {Name: "relative", Type: "b", Direction: "in"}, {Name: "interactive", Type: "b", Direction: "in"}}},
		{Name: "SetTimezone", Args: []introspect.Arg{{Name: "timezone", Type: "s", Direction: "in"}, {Name: "interactive", Type: "b", Direction: "in"}}},
		{Name: "SetLocalRTC", Args: []introspect.Arg{{Name: "local_rtc", Type: "b", Direction: "in"}, {Name: "fix_system", Type: "b", Direction: "in"}, {Name: "interactive", Type: "b", Direction: "in"}}},
		{Name: "SetNTP", Args: []introspect.Arg{{Name: "use_ntp", Type: "b", Direction: "in"}, {Name: "interactive", Type: "b", Direction: "in"}}},
		{Name: "ListTimezones", Args: []introspect.Arg{{Name: "timezones", Type: "as", Direction: "out"}}},
	},
	Properties: []introspect.Property{
		{Name: "Timezone", Type: "s", Access: "read"},
		{Name: "LocalRTC", Type: "b", Access: "read"},
		{Name: "CanNTP", Type: "b", Access: "read"},
		{Name: "NTP", Type: "b", Access: "read"},
		{Name: "NTPSynchronized", Type: "b", Access: "read"},
		{Name: "TimeUSec", Type: "t", Access: "read"},
		{Name: "RTCTimeUSec", Type: "t", Access: "read"},
	},
}

var propsIntrospection = introspect.Interface{
	Name: propsIface,
	Methods: []introspect.Method{
		{Name: "Get", Args: []introspect.Arg{{Name: "interface_name", Type: "s", Direction: "in"}, {Name: "property_name", Type: "s", Direction: "in"}, {Name: "value", Type: "v", Direction: "out"}}},
		{Name: "GetAll", Args: []introspect.Arg{{Name: "interface_name", Type: "s", Direction: "in"}, {Name: "properties", Type: "a{sv}", Direction: "out"}}},
		{Name: "Set", Args: []introspect.Arg{{Name: "interface_name", Type: "s", Direction: "in"}, {Name: "property_name", Type: "s", Direction: "in"}, {Name: "value", Type: "v", Direction: "in"}}},
	},
	Signals: []introspect.Signal{
		{Name: "PropertiesChanged", Args: []introspect.Arg{{Name: "interface_name", Type: "s"}, {Name: "changed_properties", Type: "a{sv}"}, {Name: "invalidated_properties", Type: "as"}}},
	},
}

func (s *Service) Export() error {
	if err := s.conn.ExportMethodTable(map[string]interface{}{
		"SetTime":       s.SetTime,
		"SetTimezone":   s.SetTimezone,
		"SetLocalRTC":   s.SetLocalRTC,
		"SetNTP":        s.SetNTP,
		"ListTimezones": s.ListTimezones,
	}, ObjectPath, Iface); err != nil {
		return err
	}
	if err := s.conn.ExportMethodTable(map[string]interface{}{
		"Get":    s.get,
		"GetAll": s.getAll,
		"Set":    s.set,
	}, ObjectPath, propsIface); err != nil {
		return err
	}
	node := &introspect.Node{Name: string(ObjectPath), Interfaces: []introspect.Interface{introspect.IntrospectData, propsIntrospection, introspection}}
	return s.conn.Export(introspect.NewIntrospectable(node), ObjectPath, "org.freedesktop.DBus.Introspectable")
}

func (s *Service) props() map[string]dbus.Variant {
	var rtc uint64
	if t, err := s.backend.RTC(); err == nil {
		if s.backend.LocalRTC() {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
		}
		if t.UnixMicro() > 0 {
			rtc = uint64(t.UnixMicro())
		}
	}
	return map[string]dbus.Variant{
		"Timezone":        dbus.MakeVariant(s.backend.Zone()),
		"LocalRTC":        dbus.MakeVariant(s.backend.LocalRTC()),
		"CanNTP":          dbus.MakeVariant(s.backend.CanNTP()),
		"NTP":             dbus.MakeVariant(s.backend.NTP()),
		"NTPSynchronized": dbus.MakeVariant(s.backend.Synchronized()),
		"TimeUSec":        dbus.MakeVariant(uint64(time.Now().UnixMicro())),
		"RTCTimeUSec":     dbus.MakeVariant(rtc),
	}
}

func (s *Service) get(iface, name string) (dbus.Variant, *dbus.Error) {
	all, err := s.getAll(iface)
	if err != nil {
		return dbus.Variant{}, err
	}
	v, ok := all[name]
	if !ok {
		return dbus.Variant{}, dbus.NewError("org.freedesktop.DBus.Error.UnknownProperty", []interface{}{name})
	}
	return v, nil
}

func (s *Service) getAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != Iface {
		return nil, dbus.NewError("org.freedesktop.DBus.Error.UnknownInterface", []interface{}{iface})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.props(), nil
}

func (s *Service) set(iface, name string, v dbus.Variant) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.PropertyReadOnly", []interface{}{name})
}

func (s *Service) changed(names ...string) {
	all := s.props()
	out := map[string]dbus.Variant{}
	for _, n := range names {
		out[n] = all[n]
	}
	s.conn.Emit(ObjectPath, propsIface+".PropertiesChanged", Iface, out, []string{})
}

func failed(err error) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.Failed", []interface{}{err.Error()})
}

var errDenied = errors.New("not authorized")

func (s *Service) authorize(sender dbus.Sender, action string, interactive bool) *dbus.Error {
	var uid uint32
	if err := s.conn.BusObject().Call("org.freedesktop.DBus.GetConnectionUnixUser", 0, string(sender)).Store(&uid); err != nil {
		return failed(err)
	}
	if uid == 0 {
		return nil
	}
	if s.hasPolkit() {
		subject := struct {
			Kind    string
			Details map[string]dbus.Variant
		}{"system-bus-name", map[string]dbus.Variant{"name": dbus.MakeVariant(string(sender))}}
		var res struct {
			Authorized bool
			Challenge  bool
			Details    map[string]string
		}
		flags := uint32(0)
		if interactive {
			flags = 1
		}
		obj := s.conn.Object(polkitName, "/org/freedesktop/PolicyKit1/Authority")
		if err := obj.Call("org.freedesktop.PolicyKit1.Authority.CheckAuthorization", 0, subject, action, map[string]string{}, flags, "").Store(&res); err == nil {
			if res.Authorized {
				return nil
			}
			return dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []interface{}{errDenied.Error()})
		}
	}
	if s.isAdmin(uid) {
		return nil
	}
	return dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []interface{}{errDenied.Error()})
}

func (s *Service) hasPolkit() bool {
	var owned bool
	if err := s.conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, polkitName).Store(&owned); err == nil && owned {
		return true
	}
	var names []string
	if err := s.conn.BusObject().Call("org.freedesktop.DBus.ListActivatableNames", 0).Store(&names); err != nil {
		return false
	}
	for _, n := range names {
		if n == polkitName {
			return true
		}
	}
	return false
}

func (s *Service) isAdmin(uid uint32) bool {
	if s.isAdminHook != nil {
		return s.isAdminHook(uid)
	}
	u, err := user.LookupId(itoa(uid))
	if err != nil {
		return false
	}
	ids, err := u.GroupIds()
	if err != nil {
		return false
	}
	for _, id := range ids {
		g, err := user.LookupGroupId(id)
		if err != nil {
			continue
		}
		for _, want := range s.adminGroups {
			if g.Name == want {
				return true
			}
		}
	}
	return false
}

func itoa(v uint32) string {
	if v == 0 {
		return "0"
	}
	var b [10]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func (s *Service) SetTime(usec int64, relative, interactive bool, sender dbus.Sender) *dbus.Error {
	if s.backend.NTP() {
		return dbus.NewError("org.freedesktop.timedate1.AutomaticTimeSyncEnabled", []interface{}{"Automatic time synchronization is enabled"})
	}
	if err := s.authorize(sender, actionTime, interactive); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	target := time.UnixMicro(usec)
	if relative {
		target = time.Now().Add(time.Duration(usec) * time.Microsecond)
	}
	if target.Year() < 1970 || target.Year() > 9999 {
		return dbus.NewError("org.freedesktop.DBus.Error.InvalidArgs", []interface{}{"time out of range"})
	}
	if err := s.backend.SetClock(target); err != nil {
		return failed(err)
	}
	s.backend.SyncRTC(s.backend.LocalRTC())
	s.changed("TimeUSec", "RTCTimeUSec")
	return nil
}

func (s *Service) SetTimezone(zone string, interactive bool, sender dbus.Sender) *dbus.Error {
	if err := s.authorize(sender, actionTimezone, interactive); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if zone == s.backend.Zone() {
		return nil
	}
	if err := s.backend.SetZone(zone); err != nil {
		return dbus.NewError("org.freedesktop.DBus.Error.InvalidArgs", []interface{}{err.Error()})
	}
	if loc, err := time.LoadLocation(zone); err == nil {
		time.Local = loc
	}
	if s.backend.LocalRTC() {
		s.backend.SyncRTC(true)
	}
	s.changed("Timezone", "RTCTimeUSec")
	return nil
}

func (s *Service) SetLocalRTC(local, fixSystem, interactive bool, sender dbus.Sender) *dbus.Error {
	if err := s.authorize(sender, actionLocalRTC, interactive); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if local == s.backend.LocalRTC() {
		return nil
	}
	if err := s.backend.SetLocalRTC(local); err != nil {
		return failed(err)
	}
	if fixSystem {
		if t, err := s.backend.RTC(); err == nil {
			loc := time.UTC
			if local {
				loc = time.Local
			}
			s.backend.SetClock(time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc))
		}
	} else {
		s.backend.SyncRTC(local)
	}
	s.changed("LocalRTC", "TimeUSec", "RTCTimeUSec")
	return nil
}

func (s *Service) SetNTP(enable, interactive bool, sender dbus.Sender) *dbus.Error {
	if err := s.authorize(sender, actionNTP, interactive); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.backend.CanNTP() {
		return dbus.NewError("org.freedesktop.timedate1.NoNTPSupport", []interface{}{"NTP not supported"})
	}
	if enable == s.backend.NTP() {
		return nil
	}
	if err := s.backend.SetNTP(enable); err != nil {
		return failed(err)
	}
	s.changed("NTP", "NTPSynchronized")
	return nil
}

func (s *Service) ListTimezones() ([]string, *dbus.Error) {
	zones, err := s.backend.ListZones()
	if err != nil {
		return nil, failed(err)
	}
	return zones, nil
}
