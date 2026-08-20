// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"path/filepath"
	"testing"
)

func TestWindowCloseAction(t *testing.T) {
	tests := []struct {
		name        string
		tray        bool
		quitting    bool
		wantHide    bool
		wantPrevent bool
	}{
		{name: "tray on hides", tray: true, wantHide: true, wantPrevent: true},
		{name: "tray off quits", tray: false},
		{name: "explicit quit", tray: true, quitting: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hide, prevent := windowCloseAction(tc.tray, tc.quitting)
			if hide != tc.wantHide || prevent != tc.wantPrevent {
				t.Fatalf("windowCloseAction(%v, %v) = (%v, %v), want (%v, %v)",
					tc.tray, tc.quitting, hide, prevent, tc.wantHide, tc.wantPrevent)
			}
		})
	}
}

func TestTrayActiveRequiresTarget(t *testing.T) {
	app := &App{
		Config: &Config{Mode: ModeRemote, RemoteURL: "https://carrel.example", Tray: true},
	}
	if app.trayActive() {
		t.Fatal("tray without target")
	}
	app.target = "https://carrel.example"
	if !app.trayActive() {
		t.Fatal("expected tray active")
	}
	app.Config.Tray = false
	if app.trayActive() {
		t.Fatal("tray disabled in config")
	}
}

func TestTrayHidesWindowAfterSignOut(t *testing.T) {
	dir := t.TempDir()
	app := &App{
		Paths: Paths{
			ConfigPath: filepath.Join(dir, "desktop.json"),
			LockPath:   filepath.Join(dir, "instance.lock"),
		},
		Config:      &Config{Mode: ModeLocal, Tray: true},
		target:      "http://127.0.0.1:9",
		trayRunning: true,
	}
	if !app.trayHidesWindow() {
		t.Fatal("connected tray session should hide")
	}
	app.beginSignOut()
	if !app.trayRunning {
		t.Fatal("sign-out must not Quit the tray")
	}
	if app.trayDone {
		t.Fatal("sign-out must not mark tray done")
	}
	if !app.trayHidesWindow() {
		t.Fatal("onboarding after tray session should still hide")
	}
}

func TestApplyTrayStopsWhenDisabled(t *testing.T) {
	app := &App{
		Config:      &Config{Mode: ModeRemote, RemoteURL: "https://carrel.example"},
		target:      "https://carrel.example",
		trayRunning: false,
	}
	app.applyTray()
	if app.trayDone {
		t.Fatal("stopTray with no running icon should not mark done")
	}
}
