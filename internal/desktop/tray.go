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

func (a *App) applyTray() {
	if a.trayActive() {
		a.maybeStartTray()
		return
	}
	a.stopTray()
}

func (a *App) maybeStartTray() {
	if !a.trayActive() {
		return
	}
	a.trayMu.Lock()
	if a.trayRunning || a.trayDone {
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
		case _, ok := <-open.ClickedCh:
			if !ok {
				return
			}
			a.showWindow()
		case _, ok := <-quit.ClickedCh:
			if !ok {
				return
			}
			a.requestQuit()
			return
		}
	}
}

func (a *App) onTrayExit() {
	a.trayMu.Lock()
	a.trayRunning = false
	a.trayDone = true
	a.trayMu.Unlock()
}

func (a *App) stopTray() {
	a.trayMu.Lock()
	running := a.trayRunning
	if running {
		a.trayDone = true
	}
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
	a.mu.Unlock()
	hide, prevent := windowCloseAction(a.trayHidesWindow(), quitting)
	if hide {
		runtime.WindowHide(ctx)
	}
	return prevent
}

func (a *App) trayHidesWindow() bool {
	a.mu.Lock()
	cfg := a.Config
	target := a.target
	a.mu.Unlock()
	a.trayMu.Lock()
	running := a.trayRunning
	a.trayMu.Unlock()
	if cfg != nil && cfg.Tray && target != "" {
		return true
	}
	// Sign-out clears config but leaves the icon up (Quit is one-shot).
	return running && cfg == nil
}

// windowCloseAction reports whether closing the window should hide it (tray on)
// or allow quit (tray off or explicit Quit).
func windowCloseAction(trayEnabled, quitting bool) (hide bool, prevent bool) {
	if quitting || !trayEnabled {
		return false, false
	}
	return true, true
}
