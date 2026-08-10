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
	"gitea.mixdep.ru/mix/carrel/internal/mail"
	"gitea.mixdep.ru/mix/carrel/internal/ratelimit"
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

	templates, err := loadTemplates()
	if err != nil {
		logger.Error("templates", "error", err)
		return 1
	}

	loginLimit := ratelimit.New(ratelimit.Options{})
	inviteLimit := ratelimit.New(ratelimit.Options{})

	mailQueue := &mail.Queue{
		Store:       st,
		Logger:      logger,
		ServiceName: "Carrel",
	}
	defer mailQueue.Close()

	srv := &handler.Server{
		BasePath:    cfg.BasePath,
		Trust:       trust,
		Sessions:    sessions,
		Store:       st,
		Templates:   templates,
		LoginLimit:  loginLimit,
		InviteLimit: inviteLimit,
		Mail:        mailQueue,
		Logger:      logger,
	}

	staticFS, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		logger.Error("static assets", "error", err)
		return 1
	}

	httpSrv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           srv.Handler(staticFS),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	mailQueue.Start(ctx, 1)
	sessions.StartSweeper(ctx, time.Minute)
	// Without this the limiter keeps one entry per address ever seen.
	go sweepLimiter(ctx, loginLimit, limiterSweep)
	go sweepLimiter(ctx, inviteLimit, limiterSweep)

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

// loadTemplates parses the embedded page templates.
func loadTemplates() (*handler.Templates, error) {
	templateFS, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		return nil, err
	}
	return handler.LoadTemplates(templateFS)
}

// limiterSweep is how often expired rate-limiter entries are dropped.
const limiterSweep = 10 * time.Minute

func sweepLimiter(ctx context.Context, l *ratelimit.Limiter, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.Sweep()
		}
	}
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
