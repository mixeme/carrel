// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"context"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"gitea.mixdep.ru/mix/carrel/internal/desktop/trayicon"
)

func (a *App) trayActive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.Config != nil && a.Config.Tray && a.target != ""
}

func (a *App) maybeStartTray() {
	if !a.trayActive() {
		return
	}
	a.trayMu.Lock()
	if a.trayRunning {
		a.trayMu.Unlock()
		return
	}
	a.trayRunning = true
	a.trayMu.Unlock()

	go systray.Run(a.onTrayReady, a.onTrayExit)
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayicon.Data)
	systray.SetTooltip("Carrel")

	open := systray.AddMenuItem("Open", "Show the Carrel window")
	quit := systray.AddMenuItem("Quit", "Quit Carrel")

	for {
		select {
		case <-open.ClickedCh:
			a.showWindow()
		case <-quit.ClickedCh:
			a.requestQuit()
			return
		}
	}
}

func (a *App) onTrayExit() {
	a.trayMu.Lock()
	a.trayRunning = false
	a.trayMu.Unlock()
}

func (a *App) stopTray() {
	a.trayMu.Lock()
	running := a.trayRunning
	a.trayMu.Unlock()
	if running {
		systray.Quit()
	}
}

func (a *App) showWindow() {
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx == nil {
		return
	}
	runtime.WindowShow(ctx)
	runtime.WindowUnminimise(ctx)
}

func (a *App) requestQuit() {
	a.mu.Lock()
	a.quitting = true
	ctx := a.ctx
	a.mu.Unlock()
	a.stopTray()
	if ctx != nil {
		runtime.Quit(ctx)
	}
}

func (a *App) onBeforeClose(ctx context.Context) (prevent bool) {
	a.mu.Lock()
	quitting := a.quitting
	tray := a.Config != nil && a.Config.Tray && a.target != ""
	a.mu.Unlock()
	if quitting || !tray {
		return false
	}
	runtime.WindowHide(ctx)
	return true
}

// windowCloseAction reports whether closing the window should hide it (tray on)
// or allow quit (tray off or explicit Quit).
func windowCloseAction(trayEnabled, quitting bool) (hide bool, prevent bool) {
	if quitting || !trayEnabled {
		return false, false
	}
	return true, true
}
