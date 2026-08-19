// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"gitea.mixdep.ru/mix/carrel/internal/desktop"
)

func main() {
	remoteURL := flag.String("remote-url", "", "Carrel instance URL (overrides desktop.json)")
	flag.Parse()

	paths, err := desktop.DefaultPaths()
	if err != nil {
		exitErr(err)
	}

	cfg, err := desktop.LoadConfig(paths.ConfigPath)
	if err != nil && !errors.Is(err, desktop.ErrNotConfigured) {
		exitErr(err)
	}

	url, err := desktop.ResolveRemoteURL(cfg, *remoteURL)
	if err != nil {
		if errors.Is(err, desktop.ErrNotConfigured) {
			exitErr(fmt.Errorf("%w: set desktop.json or pass -remote-url", err))
		}
		if cfg != nil && cfg.Mode == desktop.ModeLocal && *remoteURL == "" {
			exitErr(desktop.ErrLocalNotImplemented)
		}
		exitErr(err)
	}

	app := &desktop.RemoteApp{Paths: paths, URL: url}
	if err := app.Run(); err != nil {
		if errors.Is(err, desktop.ErrAlreadyRunning) {
			fmt.Fprintln(os.Stderr, "Carrel is already running.")
			os.Exit(2)
		}
		exitErr(err)
	}
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
