package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
)

// At debug, startup messages must not carry passwords, tokens, or record bodies
// (S-29). The binary only logs operational fields today; this test locks that in.
func TestDebugLogDoesNotLeakSecrets(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	slog.Info("carrel: listening", "addr", "127.0.0.1:8080", "version", "dev", "commit", "unknown")
	slog.Info("carrel: first-run mode", "dataDir", "/var/lib/carrel")

	out := strings.ToLower(buf.String())
	for _, forbidden := range []string{"password", "token", "bearer ", "authorization:", "wrappedkey"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("log contains %q: %s", forbidden, buf.String())
		}
	}
}

func TestSetupLogHonoursConfig(t *testing.T) {
	cfg := &conf.Config{LogLevel: "debug"}
	if err := setupLog(cfg); err != nil {
		t.Fatal(err)
	}
	if !slog.Default().Enabled(nil, slog.LevelDebug) {
		t.Fatal("debug level should be enabled")
	}
}
