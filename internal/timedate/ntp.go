package timedate

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/singularityos-lab/libsingularity/src/system/core/go/sintysystem"
)

const ntpService = "chrony.service"

var controlSocket = "/run/atom/control.sock"

var ErrNoServiceManager = errors.New("the sinit control socket is not present")

func IsSynchronized() bool {
	s, err := sintysystem.Clock()
	return err == nil && s.Synchronized
}

func IsNTPActive() bool {
	if state, err := sintysystem.ServiceState(ntpService); err == nil {
		return state == "active"
	}
	return processRunning("chronyd")
}

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

func HasServiceManager() bool {
	_, err := os.Stat(controlSocket)
	return err == nil
}

func SetNTP(enable bool) error {
	if !HasServiceManager() {
		return ErrNoServiceManager
	}
	action := "stop"
	if enable {
		action = "start"
	}
	return sintysystem.ServiceControl(action, ntpService)
}
