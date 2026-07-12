package timedate

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// staUnsync is the kernel timex STA_UNSYNC flag: set while no time source has
// disciplined the clock. It is not exported by x/sys/unix.
const staUnsync = 0x0040

// ntpService is the atomctl unit that provides network time on SintyOS.
const ntpService = "chrony.service"

// ErrNoAtomctl is returned by SetNTP when atomctl is not present, e.g. on a
// developer host that is not running sinit.
var ErrNoAtomctl = errors.New("atomctl not found on PATH")

// IsSynchronized reports whether the kernel clock has been disciplined by a
// time source. It reads the kernel's own timex status via adjtimex, so it
// reflects reality regardless of which daemon (chrony, ntpd, ...) is running
// and needs no external binary. This is the same signal timedatectl surfaces
// as "System clock synchronized".
func IsSynchronized() bool {
	var tx unix.Timex
	state, err := unix.Adjtimex(&tx)
	if err != nil {
		return false
	}
	if state == unix.TIME_ERROR {
		return false
	}
	return tx.Status&staUnsync == 0
}

// IsNTPActive reports whether a network-time daemon is currently running. It
// prefers chrony's own answer (`chronyc tracking` succeeds only when chronyd is
// live) and falls back to scanning /proc for the chronyd process, so it works
// even where the chronyc client is not installed.
func IsNTPActive() bool {
	if path, err := exec.LookPath("chronyc"); err == nil {
		if err := exec.Command(path, "tracking").Run(); err == nil {
			return true
		}
	}
	return processRunning("chronyd")
}

// processRunning reports whether any process has the given comm name.
func processRunning(name string) bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() || !isAllDigits(e.Name()) {
			continue
		}
		comm, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(comm)) == name {
			return true
		}
	}
	return false
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// HasAtomctl reports whether the atomctl service-manager CLI is available.
func HasAtomctl() bool {
	_, err := exec.LookPath("atomctl")
	return err == nil
}

// SetNTP enables or disables network time by starting or stopping chrony
// through atomctl, the SintyOS service manager. It returns ErrNoAtomctl when
// atomctl is absent so the caller can print a clear no-op message. Root only.
func SetNTP(enable bool) error {
	atomctl, err := exec.LookPath("atomctl")
	if err != nil {
		return ErrNoAtomctl
	}
	action := "stop"
	if enable {
		action = "start"
	}
	cmd := exec.Command(atomctl, action, ntpService)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("atomctl %s %s: %s", action, ntpService, msg)
	}
	return nil
}
