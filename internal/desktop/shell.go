// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"context"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	carrelfrontend "gitea.mixdep.ru/mix/carrel/frontend"
)

const singleInstanceID = "gitea.mixdep.ru.mix.carrel.desktop"

// Run starts the Wails window. First run shows onboarding; a saved desktop.json
// or -remote-url / -local skips it. Blocks until the window closes.
func Run(a *App) error {
	if err := prepareWebviewProfile(a.Paths); err != nil {
		return err
	}
	return wails.Run(&options.App{
		Title:     "Carrel",
		Width:     1280,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets:     carrelfrontend.Assets(),
			Middleware: a.assetMiddleware,
		},
		Bind: []interface{}{a},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				a.mu.Lock()
				ctx := a.ctx
				a.mu.Unlock()
				if ctx != nil {
					runtime.WindowShow(ctx)
					runtime.WindowUnminimise(ctx)
				}
			},
		},
		OnStartup:     a.startup,
		OnDomReady:    a.domReady,
		OnShutdown:    a.shutdown,
		OnBeforeClose: a.onBeforeClose,
		Windows: &windows.Options{
			WebviewUserDataPath: a.Paths.WebviewDataDir,
		},
		Linux: &linux.Options{
			WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
			ProgramName:      "carrel-desktop",
		},
		BackgroundColour: &options.RGBA{R: 250, G: 249, B: 247, A: 255},
	})
}

func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.pid = os.Getpid()
	a.mu.Unlock()

	if err := a.acquireOrFocus(); err != nil {
		runtime.LogError(ctx, err.Error())
		runtime.Quit(ctx)
		return
	}
	if err := a.autoConnect(ctx); err != nil {
		a.setErr(err)
		runtime.LogError(ctx, err.Error())
	}
	a.applyTray()
}

func (a *App) domReady(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	target := a.target
	a.mu.Unlock()
	if target == "" {
		return
	}
	a.navigate(target)
}

func (a *App) shutdown(ctx context.Context) {
	a.stopWatch()
	a.stopTray()
	a.stopSidecar()
	a.mu.Lock()
	pid := a.pid
	lockPath := a.Paths.LockPath
	a.mu.Unlock()
	_ = RemoveLock(lockPath, pid)
}
