// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"fmt"
	"os"
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

	shell := &windowShell{
		paths:      a.Paths,
		targetURL:  a.URL,
		onShutdown: func() { _ = RemoveLock(a.Paths.LockPath, a.pid) },
	}
	return shell.run()
}

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
