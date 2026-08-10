// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/config"
	"gitea.mixdep.ru/mix/carrel/internal/session"
	"gitea.mixdep.ru/mix/carrel/internal/store"
	"gitea.mixdep.ru/mix/carrel/internal/web"
	"gitea.mixdep.ru/mix/carrel/internal/web/handler"
)

var (
	version = "dev"
	commit  = "unknown"
)

// shutdownGrace is how long in-flight requests get to finish before the
// process stops and the keyring is wiped (§18).
const shutdownGrace = 15 * time.Second

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel(cfg.LogLevel)}))
	slog.SetDefault(logger)

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		logger.Error("open store", "error", err)
		return 1
	}
	defer func() { _ = st.Close() }()

	trust, err := handler.NewProxyTrust(cfg.TrustedProxies)
	if err != nil {
		logger.Error("trusted proxies", "error", err)
		return 1
	}

	settings := st.Settings()
	sessions := session.New(session.Options{
		Idle:     settings.SessionIdle(),
		Absolute: settings.SessionAbsolute(),
	})

	srv := &handler.Server{
		BasePath: cfg.BasePath,
		Trust:    trust,
		Sessions: sessions,
		Logger:   logger,
	}

	mux, err := routes(srv)
	if err != nil {
		logger.Error("routes", "error", err)
		return 1
	}

	httpSrv := &http.Server{
		Addr: cfg.Addr(),
		Handler: handler.Chain(mux,
			handler.Recover(logger),
			handler.SecurityHeaders(trust),
			handler.MaxBody(handler.DefaultMaxBody),
			srv.LoadSession,
		),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	sessions.StartSweeper(ctx, time.Minute)

	logger.Info("carrel starting",
		"version", version,
		"commit", commit,
		"addr", cfg.Addr(),
		"data_dir", cfg.DataDir,
		"setup_required", st.NeedsBootstrap(),
	)

	errc := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		logger.Error("server error", "error", err)
		return 1
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	shutdownErr := httpSrv.Shutdown(shutdownCtx)

	// Requests are done: no session is in use any more, so every DEK can go
	// (§24.6). StartSweeper does the same on its way out; doing it here too
	// makes the ordering explicit rather than a race with the goroutine.
	sessions.Close()

	if shutdownErr != nil {
		logger.Error("shutdown error", "error", shutdownErr)
		return 1
	}
	return 0
}

// routes wires the endpoints that exist so far. The screens themselves arrive
// with the later stages; what is fixed here is the shape — the probe and the
// static assets answer on their own, and everything that renders a page or
// takes a form goes through the CSRF check.
func routes(srv *handler.Server) (*http.ServeMux, error) {
	staticFS, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		return nil, err
	}

	// Pages: setup, login, profile, admin — added by the later stages.
	pages := http.NewServeMux()

	mux := http.NewServeMux()
	// The probe and the assets need no session and no token; issuing a CSRF
	// cookie on every health check would be pure noise.
	mux.HandleFunc("GET "+srv.Path("/healthz"), handler.Health)
	mux.Handle("GET "+srv.Path("/static/"),
		http.StripPrefix(srv.Path("/static/"), http.FileServer(http.FS(staticFS))))
	mux.Handle(srv.Path("/"), handler.Chain(pages, srv.CSRF))
	return mux, nil
}

func logLevel(name string) slog.Level {
	switch name {
	case config.LogDebug:
		return slog.LevelDebug
	case config.LogWarn:
		return slog.LevelWarn
	case config.LogError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
