package timedate

import (
	"fmt"
	"strings"
	"time"
)

// Status is a snapshot of the system's time configuration, mirroring the fields
// timedatectl prints.
type Status struct {
	Local        time.Time
	Universal    time.Time
	RTC          time.Time // zero value if the RTC could not be read
	RTCErr       error
	Zone         string
	Synchronized bool
	NTPActive    bool
	LocalRTC     bool
}

// Collect gathers the current time state. It never fails as a whole: a field
// that cannot be read (typically the RTC without permission) is recorded via
// RTCErr and rendered as "n/a", so `status` always prints something useful.
func Collect() Status {
	now := time.Now()
	cfg, _ := LoadConfig()

	s := Status{
		Local:        now.Local(),
		Universal:    now.UTC(),
		Zone:         CurrentZone(),
		Synchronized: IsSynchronized(),
		NTPActive:    IsNTPActive(),
		LocalRTC:     cfg.LocalRTC,
	}
	if rtc, err := ReadRTC(); err != nil {
		s.RTCErr = err
	} else {
		s.RTC = rtc
	}
	return s
}

// String renders the status in timedatectl's exact layout.
func (s Status) String() string {
	var b strings.Builder

	field := func(label, value string) {
		fmt.Fprintf(&b, "%25s: %s\n", label, value)
	}

	field("Local time", s.Local.Format("Mon 2006-01-02 15:04:05 MST"))
	field("Universal time", s.Universal.Format("Mon 2006-01-02 15:04:05 UTC"))
	if s.RTCErr != nil {
		field("RTC time", "n/a")
	} else {
		field("RTC time", s.RTC.Format("Mon 2006-01-02 15:04:05"))
	}
	field("Time zone", s.zoneField())
	field("System clock synchronized", yesNo(s.Synchronized))
	field("NTP service", activeInactive(s.NTPActive))
	field("RTC in local TZ", yesNo(s.LocalRTC))

	return b.String()
}

// zoneField formats the time-zone line as "Europe/Rome (CEST, +0200)".
func (s Status) zoneField() string {
	abbr, offset := s.Local.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	hh := offset / 3600
	mm := (offset % 3600) / 60
	return fmt.Sprintf("%s (%s, %s%02d%02d)", s.Zone, abbr, sign, hh, mm)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func activeInactive(b bool) string {
	if b {
		return "active"
	}
	return "inactive"
}
