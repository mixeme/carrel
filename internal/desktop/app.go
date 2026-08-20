// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"gitea.mixdep.ru/mix/carrel/internal/desktop/sidecar"
)

const watchInterval = 400 * time.Millisecond

// App is the desktop host: onboarding, Remote/Local session, and sign-out.
type App struct {
	Paths               Paths
	Config              *Config
	Version             string
	SidecarPath         string
	SkipSidecarDownload bool
	RemoteOverride      string
	ForceLocal          bool

	ctx         context.Context
	pid         int
	mu          sync.Mutex
	trayMu      sync.Mutex
	sidecarM    sync.Mutex
	trayRunning bool
	quitting    bool
	shellURL    string
	target      string
	watchOff    context.CancelFunc
	lastErr     string

	sup     *Supervisor
	running *RunningSidecar
}

// Info is the onboarding page's first read of host state.
type Info struct {
	DataDir         string `json:"dataDir"`
	NeedsOnboarding bool   `json:"needsOnboarding"`
	Error           string `json:"error,omitempty"`
}

// Info reports whether the webview should stay on onboarding.
func (a *App) Info() Info {
	a.mu.Lock()
	defer a.mu.Unlock()
	return Info{
		DataDir:         a.Paths.DataDir,
		NeedsOnboarding: a.target == "",
		Error:           a.lastErr,
	}
}

// RememberShell stores the Wails origin so sign-out can navigate back to it.
func (a *App) RememberShell(href string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if href != "" {
		a.shellURL = href
	}
}

// ConnectLocal persists Local mode, starts the sidecar, and opens it.
func (a *App) ConnectLocal(tray bool) error {
	cfg := &Config{Mode: ModeLocal, Tray: tray}
	if err := a.persist(cfg); err != nil {
		return err
	}
	ctx := a.workCtx()
	if err := a.startSidecar(ctx); err != nil {
		a.setErr(err)
		return err
	}
	a.mu.Lock()
	url := a.target
	a.mu.Unlock()
	a.navigate(url)
	a.maybeStartTray()
	return nil
}

// ConnectRemote persists Remote mode, stops a local sidecar, and opens the URL.
func (a *App) ConnectRemote(raw string, tray bool) error {
	url, err := NormalizeRemoteURL(raw)
	if err != nil {
		return err
	}
	cfg := &Config{Mode: ModeRemote, RemoteURL: url, Tray: tray}
	if err := a.persist(cfg); err != nil {
		return err
	}
	a.stopSidecar()
	a.mu.Lock()
	a.target = url
	a.lastErr = ""
	pid := a.pid
	lockPath := a.Paths.LockPath
	a.mu.Unlock()
	if pid > 0 {
		_ = WriteLock(lockPath, InstanceLock{PID: pid, Mode: ModeRemote})
	}
	a.navigate(url)
	a.maybeStartTray()
	return nil
}

func (a *App) persist(cfg *Config) error {
	if err := SaveConfig(a.Paths.ConfigPath, cfg); err != nil {
		return err
	}
	a.mu.Lock()
	a.Config = cfg
	a.lastErr = ""
	a.mu.Unlock()
	return nil
}

func (a *App) workCtx() context.Context {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func (a *App) setErr(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err == nil {
		a.lastErr = ""
		return
	}
	a.lastErr = err.Error()
}

func (a *App) navigate(url string) {
	if url == "" {
		return
	}
	a.startWatch()
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx == nil {
		return
	}
	runtime.WindowExecJS(ctx, fmt.Sprintf(`window.location.replace(%s);`, jsString(url)))
}

func (a *App) startWatch() {
	a.mu.Lock()
	ctx := a.ctx
	if ctx == nil || a.watchOff != nil {
		a.mu.Unlock()
		return
	}
	wctx, cancel := context.WithCancel(ctx)
	a.watchOff = cancel
	signOutURL := shellSignOutURL(a.shellURL)
	a.mu.Unlock()

	go a.watchLoop(wctx, signOutURL)
}

func (a *App) stopWatch() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.watchOff != nil {
		a.watchOff()
		a.watchOff = nil
	}
}

func (a *App) watchLoop(ctx context.Context, signOutURL string) {
	script := signOutWatchScript(signOutURL)
	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.mu.Lock()
			rt := a.ctx
			a.mu.Unlock()
			if rt == nil {
				continue
			}
			runtime.WindowExecJS(rt, script)
		}
	}
}

func (a *App) beginSignOut() (*Supervisor, *RunningSidecar) {
	a.stopTray()
	a.stopWatch()
	a.mu.Lock()
	a.target = ""
	a.Config = nil
	a.lastErr = ""
	path := a.Paths.ConfigPath
	pid := a.pid
	lockPath := a.Paths.LockPath
	a.mu.Unlock()
	_ = ClearConfig(path)
	if pid > 0 {
		_ = WriteLock(lockPath, InstanceLock{PID: pid, Mode: ModeSetup})
	}
	return a.detachSidecar()
}

func (a *App) applySignOut() {
	sup, running := a.beginSignOut()
	if sup != nil {
		_ = sup.Stop(running)
	}
}

func (a *App) startSidecar(ctx context.Context) error {
	if err := sidecar.Ensure(ctx, sidecar.EnsureOptions{
		Paths: sidecar.InstallPathsFrom(
			a.Paths.InstallDir,
			a.Paths.SidecarPath,
			a.Paths.VersionPath,
		),
		Version:      a.Version,
		OverridePath: a.SidecarPath,
		SkipDownload: a.SkipSidecarDownload,
	}); err != nil {
		return err
	}

	sidecarPath := a.SidecarPath
	if sidecarPath == "" {
		sidecarPath = a.Paths.SidecarPath
	}

	a.sidecarM.Lock()
	if a.sup != nil && a.running != nil {
		_ = a.sup.Stop(a.running)
		a.running = nil
	}
	sup := &Supervisor{
		SidecarPath: sidecarPath,
		DataDir:     a.Paths.DataDir,
		Bind:        loopbackBind,
	}
	running, err := sup.Start(ctx)
	if err != nil {
		a.sidecarM.Unlock()
		return err
	}
	a.sup = sup
	a.running = running
	port := running.Port
	a.sidecarM.Unlock()

	url := LocalBaseURL(port)
	a.mu.Lock()
	a.target = url
	a.lastErr = ""
	pid := a.pid
	lockPath := a.Paths.LockPath
	a.mu.Unlock()
	if pid > 0 {
		if err := WriteLock(lockPath, InstanceLock{PID: pid, Mode: ModeLocal, Port: port}); err != nil {
			a.stopSidecar()
			a.mu.Lock()
			a.target = ""
			a.mu.Unlock()
			return err
		}
	}
	return nil
}

func (a *App) detachSidecar() (*Supervisor, *RunningSidecar) {
	a.sidecarM.Lock()
	defer a.sidecarM.Unlock()
	sup, running := a.sup, a.running
	a.running = nil
	return sup, running
}

func (a *App) stopSidecar() {
	sup, running := a.detachSidecar()
	if sup != nil {
		_ = sup.Stop(running)
	}
}

func (a *App) initialLockMode() Mode {
	if ResolveLocalMode(a.Config, a.ForceLocal, a.RemoteOverride) {
		return ModeLocal
	}
	if _, err := ResolveRemoteURL(a.Config, a.RemoteOverride); err == nil {
		return ModeRemote
	}
	return ModeSetup
}

func (a *App) autoConnect(ctx context.Context) error {
	if ResolveLocalMode(a.Config, a.ForceLocal, a.RemoteOverride) {
		return a.startSidecar(ctx)
	}
	url, err := ResolveRemoteURL(a.Config, a.RemoteOverride)
	if err != nil {
		return nil
	}
	a.mu.Lock()
	a.target = url
	a.mu.Unlock()
	return nil
}

func (a *App) acquireOrFocus() error {
	if a.pid == 0 {
		a.pid = os.Getpid()
	}
	alive, existing, err := HolderAlive(a.Paths.LockPath)
	if err != nil {
		return err
	}
	if alive && existing.PID != a.pid {
		return fmt.Errorf("%w (pid %d)", ErrAlreadyRunning, existing.PID)
	}
	return AcquireLock(a.Paths.LockPath, InstanceLock{PID: a.pid, Mode: a.initialLockMode()})
}
