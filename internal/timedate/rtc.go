package timedate

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// rtcDevices lists the RTC character devices to try, most specific first.
var rtcDevices = []string{"/dev/rtc0", "/dev/rtc"}

// ReadRTC returns the current hardware-clock reading. The RTC has no timezone
// of its own; whether the stored value denotes UTC or local time is a policy
// decision (Config.LocalRTC), so this returns the raw wall-clock fields as a
// time.Time in UTC and lets the caller interpret them.
//
// It reads /dev/rtcN directly via RTC_RD_TIME and falls back to `hwclock -r`
// only if no RTC device can be opened.
func ReadRTC() (time.Time, error) {
	for _, dev := range rtcDevices {
		f, err := os.Open(dev)
		if err != nil {
			continue
		}
		rt, err := unix.IoctlGetRTCTime(int(f.Fd()))
		f.Close()
		if err != nil {
			continue
		}
		return rtcToTime(rt), nil
	}
	return hwclockRead()
}

// WriteRTC stores wall into the hardware clock. wall carries the raw clock
// fields to program (already converted to UTC or local per policy by the
// caller); only its calendar fields are used. Root only.
func WriteRTC(wall time.Time) error {
	rt := timeToRTC(wall)
	var lastErr error
	for _, dev := range rtcDevices {
		f, err := os.OpenFile(dev, os.O_RDONLY, 0)
		if err != nil {
			lastErr = err
			continue
		}
		err = unix.IoctlSetRTCTime(int(f.Fd()), &rt)
		f.Close()
		if err == nil {
			return nil
		}
		lastErr = err
	}
	if err := hwclockWrite(wall); err == nil {
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no RTC device available")
	}
	return fmt.Errorf("write RTC: %w", lastErr)
}

func rtcToTime(rt *unix.RTCTime) time.Time {
	return time.Date(
		int(rt.Year)+1900,
		time.Month(rt.Mon+1),
		int(rt.Mday),
		int(rt.Hour),
		int(rt.Min),
		int(rt.Sec),
		0, time.UTC,
	)
}

func timeToRTC(t time.Time) unix.RTCTime {
	return unix.RTCTime{
		Sec:  int32(t.Second()),
		Min:  int32(t.Minute()),
		Hour: int32(t.Hour()),
		Mday: int32(t.Day()),
		Mon:  int32(t.Month()) - 1,
		Year: int32(t.Year()) - 1900,
		Wday: int32(t.Weekday()),
		Yday: int32(t.YearDay()) - 1,
	}
}

func hwclockRead() (time.Time, error) {
	out, err := exec.Command("hwclock", "-r", "--noadjfile", "--utc").Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("read RTC: no accessible /dev/rtc device and hwclock failed: %w", err)
	}
	s := strings.TrimSpace(string(out))
	// hwclock prints e.g. "2026-07-12 09:15:03.123456+00:00"; strip sub-second
	// and offset for a lenient parse.
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = s[:i]
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse hwclock output %q: %w", s, err)
	}
	return t, nil
}

func hwclockWrite(wall time.Time) error {
	arg := wall.Format("2006-01-02 15:04:05")
	return exec.Command("hwclock", "--set", "--date", arg, "--noadjfile", "--utc").Run()
}
