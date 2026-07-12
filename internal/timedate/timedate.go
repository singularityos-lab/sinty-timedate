// Package timedate implements the system-facing logic behind sinty-timedate:
// reading and setting the system clock, timezone, RTC, and network-time state.
//
// It talks to the kernel and to chrony/atomctl directly and never depends on
// systemd or D-Bus, so it works as a drop-in for timedatectl on a de-systemd
// system where PID 1 is sinit and services are driven by atomctl.
package timedate

const (
	// LocaltimePath is the symlink the kernel-agnostic libc reads for the
	// local timezone.
	LocaltimePath = "/etc/localtime"
	// TimezonePath holds the canonical zone name in plain text.
	TimezonePath = "/etc/timezone"
	// ZoneinfoDir is the tzdata database root.
	ZoneinfoDir = "/usr/share/zoneinfo"
)

// ConfigPath persists sinty-timedate's own settings (currently only the
// local-RTC preference, which nothing else on the system records). It is a var
// so tests can point it at a scratch file.
var ConfigPath = "/etc/sinty-timedate.conf"

// RequireRoot returns an error suitable for a non-root invocation of a
// privileged verb.
func RequireRoot(verb string) error {
	return &PermError{Verb: verb}
}

// PermError signals that a privileged operation was attempted without root.
type PermError struct{ Verb string }

func (e *PermError) Error() string {
	return "operation " + e.Verb + " requires root (EUID 0)"
}
