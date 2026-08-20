// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func TestPickFreePort(t *testing.T) {
	port, err := PickFreePort(loopbackBind)
	if err != nil {
		t.Fatalf("PickFreePort() error: %v", err)
	}
	if port <= 0 {
		t.Fatalf("port = %d", port)
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(loopbackBind, strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("port %d not bindable: %v", port, err)
	}
	_ = ln.Close()
}

func TestLocalBaseURL(t *testing.T) {
	if got := LocalBaseURL(51734); got != "http://127.0.0.1:51734" {
		t.Fatalf("got %q", got)
	}
}

func TestSupervisorLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("builds fake sidecar")
	}
	fake := buildFakeSidecar(t)
	dataDir := filepath.Join(t.TempDir(), "data")

	sup := &Supervisor{
		SidecarPath: fake,
		DataDir:     dataDir,
		Bind:        loopbackBind,
		Grace:       2 * time.Second,
	}
	running, err := sup.Start(t.Context())
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if running.Port <= 0 {
		t.Fatalf("port = %d", running.Port)
	}

	resp, err := http.Get(LocalBaseURL(running.Port) + "/healthz")
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d", resp.StatusCode)
	}

	if err := sup.Stop(running); err != nil {
		t.Fatalf("Stop() error: %v", err)
	}
}

func TestSupervisorMissingSidecar(t *testing.T) {
	sup := &Supervisor{
		SidecarPath: filepath.Join(t.TempDir(), "missing"),
		DataDir:     t.TempDir(),
	}
	_, err := sup.Start(t.Context())
	if !errors.Is(err, ErrSidecarMissing) {
		t.Fatalf("Start() = %v, want ErrSidecarMissing", err)
	}
}

func TestResolveLocalMode(t *testing.T) {
	cfg := &Config{Mode: ModeLocal}
	if !ResolveLocalMode(cfg, false, "") {
		t.Fatal("expected local")
	}
	if ResolveLocalMode(cfg, false, "https://x") {
		t.Fatal("remote override forces remote")
	}
	if !ResolveLocalMode(nil, true, "") {
		t.Fatal("-local forces local")
	}
}

func TestDefaultSidecarStartIgnoresCanceledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("builds fake sidecar")
	}
	fake := buildFakeSidecar(t)
	port, err := PickFreePort(loopbackBind)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	env := append(os.Environ(),
		"CARREL_PORT="+strconv.Itoa(port),
		"CARREL_BIND="+loopbackBind,
	)
	cmd, err := defaultSidecarStart(ctx, fake, env, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = stopProcess(cmd, 2*time.Second) })

	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, err := http.Get(LocalBaseURL(port) + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("sidecar exited after a canceled start context")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func buildFakeSidecar(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "fake_carrel")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", out, "./testdata/fakesidecar")
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake sidecar: %v\n%s", err, outBytes)
	}
	return out
}
