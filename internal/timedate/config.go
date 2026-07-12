package timedate

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Config holds sinty-timedate's persisted preferences.
type Config struct {
	// LocalRTC is true when the hardware clock is kept in local time rather
	// than UTC.
	LocalRTC bool
}

// LoadConfig reads ConfigPath. A missing file yields the zero value (RTC in
// UTC), matching the default assumption of every sane installation.
func LoadConfig() (Config, error) {
	var c Config
	f, err := os.Open(ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return c, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) == "LOCAL_RTC" {
			c.LocalRTC = parseBool(strings.TrimSpace(val))
		}
	}
	return c, sc.Err()
}

// Save writes the config atomically.
func (c Config) Save() error {
	body := fmt.Sprintf("# Managed by sinty-timedate\nLOCAL_RTC=%t\n", c.LocalRTC)
	return writeFileAtomic(ConfigPath, []byte(body), 0o644)
}

func parseBool(s string) bool {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// ParseBoolArg parses a CLI true/false argument, rejecting anything else.
func ParseBoolArg(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("expected a boolean (true/false), got %q", s)
	}
}
