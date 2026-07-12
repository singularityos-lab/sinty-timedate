package timedate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CurrentZone returns the configured timezone name (e.g. "Europe/Rome"),
// resolved the same way libc and timedatectl do: first the /etc/localtime
// symlink target, then /etc/timezone, then a scan of zoneinfo for a file whose
// contents match /etc/localtime. It returns "n/a" when nothing resolves.
func CurrentZone() string {
	if target, err := os.Readlink(LocaltimePath); err == nil {
		if z := zoneFromPath(target); z != "" {
			return z
		}
	}
	if data, err := os.ReadFile(TimezonePath); err == nil {
		if z := strings.TrimSpace(string(data)); z != "" {
			return z
		}
	}
	// /etc/localtime may be a plain copy of a zoneinfo file (no symlink and no
	// /etc/timezone). Match it by content against the database.
	if z := zoneByContent(); z != "" {
		return z
	}
	return "n/a"
}

// zoneFromPath extracts the zone name from a path that points into the zoneinfo
// database, handling both absolute and "../usr/share/zoneinfo/..." relative
// symlink targets.
func zoneFromPath(target string) string {
	idx := strings.Index(target, "zoneinfo/")
	if idx < 0 {
		return ""
	}
	return target[idx+len("zoneinfo/"):]
}

func zoneByContent() string {
	want, err := os.ReadFile(LocaltimePath)
	if err != nil || len(want) == 0 {
		return ""
	}
	var found string
	filepath.WalkDir(ZoneinfoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || found != "" {
			return nil
		}
		name, ok := zoneName(path)
		if !ok {
			return nil
		}
		got, err := os.ReadFile(path)
		if err == nil && string(got) == string(want) {
			found = name
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// ListZones enumerates every valid zone under ZoneinfoDir, sorted, matching the
// set timedatectl list-timezones prints.
func ListZones() ([]string, error) {
	var zones []string
	err := filepath.WalkDir(ZoneinfoDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			// Skip alias symlinks (e.g. legacy names); timedatectl lists the
			// canonical zone files only.
			return nil
		}
		name, ok := zoneName(path)
		if !ok {
			return nil
		}
		zones = append(zones, name)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(zones)
	return zones, nil
}

// zoneName maps a zoneinfo file path to its zone name and reports whether the
// path is an actual timezone (not database metadata like posix/, right/,
// zone.tab, iso3166.tab, tzdata.zi, or a lowercase/pseudo entry).
func zoneName(path string) (string, bool) {
	rel, err := filepath.Rel(ZoneinfoDir, path)
	if err != nil {
		return "", false
	}
	switch {
	case strings.HasPrefix(rel, "posix/"),
		strings.HasPrefix(rel, "right/"),
		strings.HasSuffix(rel, ".tab"),
		strings.HasSuffix(rel, ".list"),
		strings.HasSuffix(rel, ".zi"),
		rel == "leapseconds",
		rel == "leap-seconds.list",
		rel == "localtime",
		rel == "Factory":
		return "", false
	}
	// Real zone names begin each component with an uppercase letter or a sign
	// (e.g. "GMT+0"); this drops files like "zone.tab" already handled above
	// and keeps "Etc/GMT-14".
	first := rel[0]
	if first >= 'a' && first <= 'z' {
		return "", false
	}
	return rel, true
}

// ValidZone reports whether name is a real zone under ZoneinfoDir. It rejects
// path traversal and any target that is not a regular zone file.
func ValidZone(name string) bool {
	if name == "" || strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
		return false
	}
	clean := filepath.Clean(name)
	if clean != name {
		return false
	}
	full := filepath.Join(ZoneinfoDir, name)
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return false
	}
	_, ok := zoneName(full)
	return ok
}

// SetZone repoints /etc/localtime at the given zone and records it in
// /etc/timezone. The symlink is written atomically (temp symlink + rename) so a
// concurrent reader never sees a missing /etc/localtime. Root only.
func SetZone(name string) error {
	if !ValidZone(name) {
		return fmt.Errorf("invalid or unknown timezone %q (see: sinty-timedate list-timezones)", name)
	}

	target := filepath.Join("..", "usr", "share", "zoneinfo", name)
	dir := filepath.Dir(LocaltimePath)
	tmp := filepath.Join(dir, ".sinty-timedate-localtime")
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return fmt.Errorf("create timezone symlink: %w", err)
	}
	if err := os.Rename(tmp, LocaltimePath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("install timezone symlink: %w", err)
	}

	if err := writeFileAtomic(TimezonePath, []byte(name+"\n"), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", TimezonePath, err)
	}
	return nil
}
