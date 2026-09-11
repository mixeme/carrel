package conf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", c.Port)
	}
	if c.DAVConnectTimeout != 10*time.Second {
		t.Fatalf("DAVConnectTimeout = %v", c.DAVConnectTimeout)
	}
	if c.DAVloomPort != 0 {
		t.Fatalf("DAVloomPort = %d, want 0 (disabled)", c.DAVloomPort)
	}
	if SessionIdleTimeout != 12*time.Hour {
		t.Fatalf("SessionIdleTimeout = %v", SessionIdleTimeout)
	}
	if MaxFormBodyBytes != 1<<20 {
		t.Fatalf("MaxFormBodyBytes = %d", MaxFormBodyBytes)
	}
}

func TestFileOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, settingsFile), []byte("CARREL_PORT=9090\nCARREL_DAVLOOM_PORT=8443\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CARREL_DATA_DIR", dir)

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 9090 {
		t.Fatalf("Port = %d, want 9090", c.Port)
	}
	if c.DAVloomPort != 8443 {
		t.Fatalf("DAVloomPort = %d, want 8443", c.DAVloomPort)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, settingsFile), []byte("CARREL_PORT=9090\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CARREL_DATA_DIR", dir)
	t.Setenv("CARREL_PORT", "3000")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != 3000 {
		t.Fatalf("Port = %d, want 3000 (env wins)", c.Port)
	}
}

func TestDAVloomEmptyDisables(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARREL_DATA_DIR", dir)
	t.Setenv("CARREL_DAVLOOM_PORT", "")

	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DAVloomPort != 0 {
		t.Fatalf("DAVloomPort = %d, want 0", c.DAVloomPort)
	}
}

// A typo in a variable is a startup error the operator can read, not a panic
// halfway through Load.
func TestBadValuesAreErrorsNotPanics(t *testing.T) {
	for _, tc := range []struct{ key, val string }{
		{"CARREL_PORT", "eighty"},
		{"CARREL_DAV_MAX_RESPONSE_BYTES", "10MiB"},
		{"CARREL_DAV_REQUEST_TIMEOUT", "30s"},
		{"CARREL_MAIL_ENABLED", "maybe"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			t.Setenv("CARREL_DATA_DIR", t.TempDir())
			t.Setenv(tc.key, tc.val)
			if _, err := Load(); err == nil {
				t.Fatalf("%s=%q: expected an error", tc.key, tc.val)
			} else if !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("error should name the variable: %v", err)
			}
		})
	}
}

// davloom gets its own listener; sharing the web port is a misconfiguration.
func TestDavloomPortMustDifferFromWebPort(t *testing.T) {
	t.Setenv("CARREL_DATA_DIR", t.TempDir())
	t.Setenv("CARREL_PORT", "8080")
	t.Setenv("CARREL_DAVLOOM_PORT", "8080")
	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a shared port")
	}

	t.Setenv("CARREL_DAVLOOM_PORT", "")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DAVloomPort != 0 {
		t.Fatalf("empty CARREL_DAVLOOM_PORT should disable davloom, got %d", c.DAVloomPort)
	}
}
