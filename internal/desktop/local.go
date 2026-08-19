// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// LocalApp runs Carrel in Local mode: start a loopback sidecar and open it in
// the desktop webview.
type LocalApp struct {
	Paths       Paths
	SidecarPath string
	pid         int
	sup         *Supervisor
	running     *RunningSidecar
}

// Run starts the local sidecar supervisor and opens the Wails window.
func (a *LocalApp) Run() error {
	a.pid = os.Getpid()
	shell := &windowShell{
		paths:      a.Paths,
		onShutdown: a.shutdown,
	}
	shell.onStartup = func(ctx context.Context) error {
		return a.startSidecar(ctx, shell)
	}
	return shell.run()
}

func (a *LocalApp) startSidecar(ctx context.Context, shell *windowShell) error {
	alive, existing, err := HolderAlive(a.Paths.LockPath)
	if err != nil {
		return err
	}
	if alive && existing.PID != a.pid {
		return fmt.Errorf("%w (pid %d)", ErrAlreadyRunning, existing.PID)
	}

	sidecar := a.SidecarPath
	if sidecar == "" {
		sidecar = a.Paths.SidecarPath
	}
	a.sup = &Supervisor{
		SidecarPath: sidecar,
		DataDir:     a.Paths.DataDir,
		Bind:        loopbackBind,
	}
	running, err := a.sup.Start(ctx)
	if err != nil {
		return err
	}
	a.running = running
	shell.targetURL = LocalBaseURL(running.Port)

	lock := InstanceLock{PID: a.pid, Mode: ModeLocal, Port: running.Port}
	if err := WriteLock(a.Paths.LockPath, lock); err != nil {
		_ = a.sup.Stop(running)
		a.running = nil
		return err
	}
	return nil
}

func (a *LocalApp) shutdown() {
	if a.sup != nil && a.running != nil {
		_ = a.sup.Stop(a.running)
		a.running = nil
	}
	_ = RemoveLock(a.Paths.LockPath, a.pid)
}

// ResolveLocalMode reports whether Local mode should run.
func ResolveLocalMode(cfg *Config, forceLocal bool, remoteOverride string) bool {
	if remoteOverride != "" {
		return false
	}
	if forceLocal {
		return true
	}
	return cfg != nil && cfg.Mode == ModeLocal
}

// ErrLocalConfig is returned when Local mode is requested without configuration.
var ErrLocalConfig = errors.New("desktop: local mode requires desktop.json or -local with sidecar")
