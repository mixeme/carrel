//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// liveCredEnv fills CARREL_* from dev/credentials.txt when unset (local dev only).
func liveCredEnv() {
	if os.Getenv("CARREL_SFTPGO_URL") != "" && os.Getenv("CARREL_BAIKAL_URL") != "" {
		return
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return
	}
	// integration/live_creds.go → carrel/server → carrel_reborn
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	path := filepath.Join(root, "dev", "credentials.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch line {
		case "Baikal DAV":
			section = "baikal"
			continue
		case "SFTPGo WebDAV":
			section = "sftpgo"
			continue
		case "":
			section = ""
			continue
		}
		switch section {
		case "baikal":
			setIfEmpty("CARREL_BAIKAL_URL", urlLine(line))
			setIfEmpty("CARREL_BAIKAL_USER", loginLine(line))
			setIfEmpty("CARREL_BAIKAL_PASS", passwordLine(line))
		case "sftpgo":
			setIfEmpty("CARREL_SFTPGO_URL", urlLine(line))
			setIfEmpty("CARREL_SFTPGO_USER", loginLine(line))
			setIfEmpty("CARREL_SFTPGO_PASS", passwordLine(line))
		}
	}
}

func setIfEmpty(key, val string) {
	if val != "" && os.Getenv(key) == "" {
		_ = os.Setenv(key, val)
	}
}

func urlLine(line string) string {
	if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
		return line
	}
	return ""
}

func loginLine(line string) string {
	const p = "login:"
	if strings.HasPrefix(line, p) {
		return strings.TrimSpace(strings.TrimPrefix(line, p))
	}
	return ""
}

func passwordLine(line string) string {
	const p = "password:"
	if strings.HasPrefix(line, p) {
		return strings.TrimSpace(strings.TrimPrefix(line, p))
	}
	return ""
}
