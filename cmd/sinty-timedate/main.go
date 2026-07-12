// Command sinty-timedate is a native replacement for systemd's timedatectl,
// built for SintyOS where PID 1 is sinit (not systemd) and services are driven
// by atomctl. The stock timedatectl refuses to run without systemd as PID 1;
// this talks to the kernel, zoneinfo, the RTC, and chrony/atomctl directly.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/singularityos-lab/sinty-timedate/internal/timedate"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return cmdStatus()
	}

	switch args[0] {
	case "-h", "--help", "help":
		usage(os.Stdout)
		return 0
	case "status":
		return cmdStatus()
	case "list-timezones":
		return cmdListTimezones()
	case "set-timezone":
		return cmdSetTimezone(args[1:])
	case "set-time":
		return cmdSetTime(args[1:])
	case "set-ntp":
		return cmdSetNTP(args[1:])
	case "set-local-rtc":
		return cmdSetLocalRTC(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "sinty-timedate: unknown command %q\n\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func cmdStatus() int {
	fmt.Print(timedate.Collect().String())
	return 0
}

func cmdListTimezones() int {
	zones, err := timedate.ListZones()
	if err != nil {
		return fail("list-timezones: %v", err)
	}
	for _, z := range zones {
		fmt.Println(z)
	}
	return 0
}

func cmdSetTimezone(args []string) int {
	if len(args) != 1 {
		return fail("set-timezone: expected exactly one ZONE argument")
	}
	if !isRoot() {
		return fail("%v", timedate.RequireRoot("set-timezone"))
	}
	if err := timedate.SetZone(args[0]); err != nil {
		return fail("set-timezone: %v", err)
	}
	return 0
}

func cmdSetTime(args []string) int {
	if len(args) != 1 {
		return fail("set-time: expected exactly one \"YYYY-MM-DD HH:MM:SS\" argument")
	}
	if !isRoot() {
		return fail("%v", timedate.RequireRoot("set-time"))
	}
	// Match timedatectl: refuse to set the clock by hand while NTP disciplines it.
	if timedate.IsNTPActive() {
		return fail("Failed to set time: Automatic time synchronization is enabled")
	}
	t, err := timedate.ParseWallTime(args[0])
	if err != nil {
		return fail("set-time: %v", err)
	}
	if err := timedate.SetSystemClock(t); err != nil {
		return fail("set-time: %v", err)
	}
	cfg, _ := timedate.LoadConfig()
	if err := timedate.SyncRTCFromSystem(cfg.LocalRTC); err != nil {
		return fail("set-time: system clock set but RTC update failed: %v", err)
	}
	return 0
}

func cmdSetNTP(args []string) int {
	if len(args) != 1 {
		return fail("set-ntp: expected true or false")
	}
	enable, err := timedate.ParseBoolArg(args[0])
	if err != nil {
		return fail("set-ntp: %v", err)
	}
	if !isRoot() {
		return fail("%v", timedate.RequireRoot("set-ntp"))
	}
	if err := timedate.SetNTP(enable); err != nil {
		if errors.Is(err, timedate.ErrNoAtomctl) {
			fmt.Fprintln(os.Stderr, "set-ntp: atomctl not found; not running under sinit, nothing to do")
			return 0
		}
		return fail("set-ntp: %v", err)
	}
	return 0
}

func cmdSetLocalRTC(args []string) int {
	if len(args) != 1 {
		return fail("set-local-rtc: expected true or false")
	}
	local, err := timedate.ParseBoolArg(args[0])
	if err != nil {
		return fail("set-local-rtc: %v", err)
	}
	if !isRoot() {
		return fail("%v", timedate.RequireRoot("set-local-rtc"))
	}
	cfg, _ := timedate.LoadConfig()
	cfg.LocalRTC = local
	if err := cfg.Save(); err != nil {
		return fail("set-local-rtc: %v", err)
	}
	// Reprogram the RTC so its stored value matches the new policy.
	if err := timedate.SyncRTCFromSystem(local); err != nil {
		return fail("set-local-rtc: preference saved but RTC update failed: %v", err)
	}
	return 0
}

func isRoot() bool { return os.Geteuid() == 0 }

func fail(format string, a ...any) int {
	fmt.Fprintf(os.Stderr, "sinty-timedate: "+format+"\n", a...)
	return 1
}

func usage(w *os.File) {
	fmt.Fprint(w, `sinty-timedate - query and set the system clock, timezone, and RTC on SintyOS

Usage:
  sinty-timedate [status]                 show the current time settings
  sinty-timedate list-timezones           list all known timezones
  sinty-timedate set-timezone ZONE        set the system timezone (root)
  sinty-timedate set-time "Y-M-D H:M:S"    set the system clock and RTC (root)
  sinty-timedate set-ntp true|false        enable/disable network time via chrony (root)
  sinty-timedate set-local-rtc true|false  keep the RTC in local time instead of UTC (root)
  sinty-timedate --help                    show this help

Privileged commands require EUID 0. Network time is driven through atomctl
(chrony.service); the clock, timezone, and RTC are read and written directly,
so this works without systemd.
`)
}
