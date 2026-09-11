package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
)

func TestIssueConsoleWipeLink(t *testing.T) {
	dir := t.TempDir()
	key, err := crypt.CreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = dir
	mgr := session.NewManager(cfg)
	svc := user.New(st, mgr, cfg)
	if err := svc.SetupFirstAdmin(context.Background(), "admin", "password123"); err != nil {
		t.Fatal(err)
	}

	bin := filepath.Join(t.TempDir(), "carrel-wipe.exe")
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "admin")
	cmd.Env = append(os.Environ(), "CARREL_DATA_DIR="+dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run: %v stderr=%q", err, stderr.String())
	}
	link := strings.TrimSpace(stdout.String())
	if !strings.HasPrefix(link, "/forgot/") {
		t.Fatalf("link = %q", link)
	}
	token := strings.TrimPrefix(link, "/forgot/")
	if len(token) < 32 {
		t.Fatalf("token too short: %d", len(token))
	}

	st2, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	snap := st2.Snapshot()
	admin := snap.UserByLogin("admin")
	if admin == nil {
		t.Fatal("admin missing")
	}
	found := false
	for _, tok := range snap.Tokens {
		if tok.Kind == "console_wipe" && tok.UserID == admin.ID && !tok.Used {
			found = true
			if time.Until(tok.ExpiresAt) > conf.ConsoleWipeLinkTTL {
				t.Fatalf("expires too far: %v", tok.ExpiresAt)
			}
		}
	}
	if !found {
		t.Fatal("console wipe token not stored")
	}
	foundAudit := false
	for _, e := range snap.Audit {
		if e.What == "user.console.wipe" && e.Whom == "admin" {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatal("audit entry missing")
	}
}
