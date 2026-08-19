// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	carrelfrontend "gitea.mixdep.ru/mix/carrel/frontend"
)

const singleInstanceID = "gitea.mixdep.ru.mix.carrel.desktop"

type windowShell struct {
	paths      Paths
	targetURL  string
	ctx        context.Context
	onStartup  func(ctx context.Context) error
	onShutdown func()
}

func (s *windowShell) run() error {
	return wails.Run(&options.App{
		Title:     "Carrel",
		Width:     1280,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: carrelfrontend.Assets(),
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				if s.ctx != nil {
					runtime.WindowShow(s.ctx)
					runtime.WindowUnminimise(s.ctx)
				}
			},
		},
		OnStartup:  s.startup,
		OnDomReady: s.domReady,
		OnShutdown: s.shutdown,
		Windows: &windows.Options{
			WebviewUserDataPath: s.paths.WebviewDataDir,
		},
		Linux: &linux.Options{
			WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
		},
		BackgroundColour: &options.RGBA{R: 250, G: 249, B: 247, A: 255},
	})
}

func (s *windowShell) startup(ctx context.Context) {
	s.ctx = ctx
	if s.onStartup == nil {
		return
	}
	if err := s.onStartup(ctx); err != nil {
		runtime.LogError(ctx, err.Error())
		runtime.Quit(ctx)
	}
}

func (s *windowShell) domReady(ctx context.Context) {
	s.ctx = ctx
	if s.targetURL == "" {
		return
	}
	script := fmt.Sprintf(`window.location.replace(%s);`, jsString(s.targetURL))
	runtime.WindowExecJS(ctx, script)
}

func (s *windowShell) shutdown(ctx context.Context) {
	if s.onShutdown != nil {
		s.onShutdown()
	}
}
