// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClearConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop.json")
	if err := SaveConfig(path, &Config{Mode: ModeLocal, Tray: true}); err != nil {
		t.Fatal(err)
	}
	if err := ClearConfig(path); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("LoadConfig after clear = %v", err)
	}
	if err := ClearConfig(path); err != nil {
		t.Fatalf("second clear: %v", err)
	}
}

func TestConnectRemotePersistsAndStopsSidecar(t *testing.T) {
	if testing.Short() {
		t.Skip("builds fake sidecar")
	}
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "desktop.json")
	dataDir := filepath.Join(dir, "data")
	fake := buildFakeSidecar(t)

	app := &App{
		Paths: Paths{ConfigPath: cfgPath, DataDir: dataDir, LockPath: filepath.Join(dir, "instance.lock")},
		pid:   os.Getpid(),
	}
	if err := SaveConfig(cfgPath, &Config{Mode: ModeLocal}); err != nil {
		t.Fatal(err)
	}
	app.Config = &Config{Mode: ModeLocal}

	sup := &Supervisor{SidecarPath: fake, DataDir: dataDir, Bind: loopbackBind, Grace: 2 * time.Second}
	running, err := sup.Start(t.Context())
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	app.sup = sup
	app.running = running
	port := running.Port

	if err := app.ConnectRemote("https://carrel.example", true); err != nil {
		t.Fatalf("ConnectRemote() error: %v", err)
	}
	got, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != ModeRemote || got.RemoteURL != "https://carrel.example" || !got.Tray {
		t.Fatalf("saved %+v", got)
	}
	if app.running != nil {
		t.Fatal("sidecar still recorded as running")
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, err := http.Get(LocalBaseURL(port) + "/healthz")
		if err != nil {
			break
		}
		_ = resp.Body.Close()
		if time.Now().After(deadline) {
			t.Fatal("sidecar still answering after ConnectRemote")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestApplySignOutClearsConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("builds fake sidecar")
	}
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "desktop.json")
	dataDir := filepath.Join(dir, "data")
	lockPath := filepath.Join(dir, "instance.lock")
	if err := SaveConfig(cfgPath, &Config{Mode: ModeLocal, Tray: true}); err != nil {
		t.Fatal(err)
	}

	fake := buildFakeSidecar(t)
	app := &App{
		Paths: Paths{ConfigPath: cfgPath, DataDir: dataDir, LockPath: lockPath},
		pid:   os.Getpid(),
	}
	sup := &Supervisor{SidecarPath: fake, DataDir: dataDir, Bind: loopbackBind, Grace: 2 * time.Second}
	running, err := sup.Start(t.Context())
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	app.sup = sup
	app.running = running
	app.target = LocalBaseURL(running.Port)
	port := running.Port

	app.applySignOut()

	if _, err := LoadConfig(cfgPath); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("config after sign-out = %v", err)
	}
	if app.target != "" {
		t.Fatalf("target still %q", app.target)
	}
	if app.running != nil {
		t.Fatal("sidecar still recorded")
	}
	lock, err := ReadLock(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if lock == nil || lock.Mode != ModeSetup {
		t.Fatalf("lock = %+v, want setup", lock)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, err := http.Get(LocalBaseURL(port) + "/healthz")
		if err != nil {
			return
		}
		_ = resp.Body.Close()
		if time.Now().After(deadline) {
			t.Fatal("sidecar still answering after sign-out")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestConnectLocalPersists(t *testing.T) {
	if testing.Short() {
		t.Skip("builds fake sidecar")
	}
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "desktop.json")
	fake := buildFakeSidecar(t)
	app := &App{
		Paths: Paths{
			ConfigPath:  cfgPath,
			DataDir:     filepath.Join(dir, "data"),
			LockPath:    filepath.Join(dir, "instance.lock"),
			InstallDir:  dir,
			SidecarPath: fake,
		},
		SidecarPath:         fake,
		SkipSidecarDownload: true,
		pid:                 os.Getpid(),
	}
	if err := app.ConnectLocal(true); err != nil {
		t.Fatalf("ConnectLocal() error: %v", err)
	}
	t.Cleanup(app.stopSidecar)
	got, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != ModeLocal || !got.Tray {
		t.Fatalf("saved %+v", got)
	}
	if app.target == "" {
		t.Fatal("target URL empty")
	}
}

func TestInfoNeedsOnboarding(t *testing.T) {
	app := &App{Paths: Paths{DataDir: "/tmp/carrel-data"}}
	info := app.Info()
	if !info.NeedsOnboarding || info.DataDir != "/tmp/carrel-data" {
		t.Fatalf("%+v", info)
	}
	app.target = "https://carrel.example"
	info = app.Info()
	if info.NeedsOnboarding {
		t.Fatal("expected configured")
	}
}

func TestWriteLockSetupMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	if err := WriteLock(path, InstanceLock{PID: os.Getpid(), Mode: ModeSetup}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != ModeSetup {
		t.Fatalf("mode %q", got.Mode)
	}
}
