package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SaveSetting writes key=value into settings.env of the data directory,
// replacing the line of that key or adding one; comments and every other
// line stay (S-28.1: the declared public address). Written to a temporary
// file and renamed, like the state. A variable of the process environment
// still wins over it at the next start.
func SaveSetting(dataDir, key, value string) error {
	if strings.ContainsAny(key+value, "\r\n") {
		return fmt.Errorf("conf: %s: a line break", key)
	}
	path := filepath.Join(dataDir, settingsFile)
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var lines []string
	done := false
	for _, line := range strings.Split(strings.TrimRight(string(old), "\n"), "\n") {
		if line == "" && len(old) == 0 {
			continue
		}
		k, _, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok && strings.TrimSpace(k) == key && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			if !done {
				lines = append(lines, key+"="+value)
				done = true
			}
			continue
		}
		lines = append(lines, line)
	}
	if !done {
		lines = append(lines, key+"="+value)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// FromEnvironment reports whether the process environment sets key: then
// settings.env does not win at the next start.
func FromEnvironment(key string) bool {
	_, ok := os.LookupEnv(key)
	return ok
}
