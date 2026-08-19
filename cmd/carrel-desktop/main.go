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

var (
	version = "0.9.0"
	commit  = "unknown"
)

func main() {
	remoteURL := flag.String("remote-url", "", "Carrel instance URL (overrides desktop.json; forces Remote mode)")
	forceLocal := flag.Bool("local", false, "force Local mode (sidecar on loopback)")
	sidecarPath := flag.String("sidecar", "", "path to carrel sidecar binary (skips download)")
	skipDownload := flag.Bool("skip-sidecar-download", false, "do not download sidecar; fail if missing")
	flag.Parse()

	paths, err := desktop.DefaultPaths()
	if err != nil {
		exitErr(err)
	}

	cfg, err := desktop.LoadConfig(paths.ConfigPath)
	if err != nil && !errors.Is(err, desktop.ErrNotConfigured) {
		exitErr(err)
	}

	if desktop.ResolveLocalMode(cfg, *forceLocal, *remoteURL) {
		app := &desktop.LocalApp{
			Paths:               paths,
			SidecarPath:         *sidecarPath,
			Version:             version,
			SkipSidecarDownload: *skipDownload,
		}
		if err := app.Run(); err != nil {
			handleRunErr(err)
		}
		return
	}

	url, err := desktop.ResolveRemoteURL(cfg, *remoteURL)
	if err != nil {
		if errors.Is(err, desktop.ErrNotConfigured) {
			exitErr(fmt.Errorf("%w: set desktop.json, pass -remote-url, or -local", err))
		}
		exitErr(err)
	}

	app := &desktop.RemoteApp{Paths: paths, URL: url}
	if err := app.Run(); err != nil {
		handleRunErr(err)
	}
}

func handleRunErr(err error) {
	if errors.Is(err, desktop.ErrAlreadyRunning) {
		fmt.Fprintln(os.Stderr, "Carrel is already running.")
		os.Exit(2)
	}
	exitErr(err)
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
