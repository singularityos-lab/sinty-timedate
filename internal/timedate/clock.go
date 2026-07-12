package timedate

import (
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

// TimeLayout is the wall-clock format accepted by set-time, matching the
// primary form timedatectl accepts.
const TimeLayout = "2006-01-02 15:04:05"

// ParseWallTime parses a "YYYY-MM-DD HH:MM:SS" string in the machine's local
// timezone, the same interpretation timedatectl uses for set-time.
func ParseWallTime(s string) (time.Time, error) {
	t, err := time.ParseInLocation(TimeLayout, s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q (want \"YYYY-MM-DD HH:MM:SS\"): %w", s, err)
	}
	return t, nil
}

// SetSystemClock sets CLOCK_REALTIME to t. Root only.
func SetSystemClock(t time.Time) error {
	ts := unix.NsecToTimespec(t.UnixNano())
	if err := unix.ClockSettime(unix.CLOCK_REALTIME, &ts); err != nil {
		return fmt.Errorf("set system clock: %w", err)
	}
	return nil
}

// SyncRTCFromSystem writes the current system time into the hardware clock,
// storing UTC or local wall-clock fields according to localRTC. Root only.
func SyncRTCFromSystem(localRTC bool) error {
	return WriteRTC(rtcWallFor(time.Now(), localRTC))
}

// rtcWallFor returns the calendar fields to program into the RTC for instant t,
// given the local-RTC policy. With localRTC the RTC holds local wall time;
// otherwise it holds UTC. The returned time.Time is a field carrier only.
func rtcWallFor(t time.Time, localRTC bool) time.Time {
	if localRTC {
		lt := t.Local()
		return time.Date(lt.Year(), lt.Month(), lt.Day(), lt.Hour(), lt.Minute(), lt.Second(), 0, time.UTC)
	}
	return t.UTC()
}
