// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	carrelfrontend "gitea.mixdep.ru/mix/carrel/frontend"
)

// RemoteApp opens a Wails window and navigates the webview to a remote Carrel
// instance. Fan-out progress uses the same SSE (or poll fallback) as a browser
// once the Carrel UI is loaded.
type RemoteApp struct {
	Paths Paths
	URL   string
	pid   int
}

// Run starts the Remote-mode desktop shell. It acquires the instance lock and
// blocks until the window closes.
func (a *RemoteApp) Run() error {
	if a.URL == "" {
		return ErrRemoteURL
	}
	normalized, err := NormalizeRemoteURL(a.URL)
	if err != nil {
		return err
	}
	a.URL = normalized
	a.pid = os.Getpid()

	lock := InstanceLock{PID: a.pid, Mode: ModeRemote}
	if err := AcquireLock(a.Paths.LockPath, lock); err != nil {
		return err
	}
	defer func() { _ = RemoveLock(a.Paths.LockPath, a.pid) }()

	app := &remoteShell{targetURL: a.URL}
	return wails.Run(&options.App{
		Title:     "Carrel",
		Width:     1280,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: carrelfrontend.Assets(),
		},
		OnStartup:  app.startup,
		OnDomReady: app.domReady,
		OnShutdown: app.shutdown,
		Windows: &windows.Options{
			WebviewUserDataPath: a.Paths.WebviewDataDir,
		},
		Linux: &linux.Options{
			WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
		},
		BackgroundColour: &options.RGBA{R: 250, G: 249, B: 247, A: 255},
	})
}

type remoteShell struct {
	ctx       context.Context
	targetURL string
}

func (s *remoteShell) startup(ctx context.Context) {
	s.ctx = ctx
}

func (s *remoteShell) domReady(ctx context.Context) {
	s.ctx = ctx
	script := fmt.Sprintf(`window.location.replace(%s);`, jsString(s.targetURL))
	runtime.WindowExecJS(ctx, script)
}

func (s *remoteShell) shutdown(ctx context.Context) {}

// ResolveRemoteURL returns the URL for Remote mode from config and an optional
// command-line override.
func ResolveRemoteURL(cfg *Config, override string) (string, error) {
	if override != "" {
		return NormalizeRemoteURL(override)
	}
	if cfg == nil {
		return "", ErrNotConfigured
	}
	if cfg.Mode != ModeRemote {
		return "", fmt.Errorf("desktop: mode %q is not remote", cfg.Mode)
	}
	return NormalizeRemoteURL(cfg.RemoteURL)
}

// ErrLocalNotImplemented is returned when desktop.json requests Local mode before
// the supervisor exists.
var ErrLocalNotImplemented = errors.New("desktop: local mode is not implemented yet")
