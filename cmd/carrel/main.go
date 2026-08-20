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
	"strconv"
	"strings"
	"syscall"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/config"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/fanout"
	"gitea.mixdep.ru/mix/carrel/internal/mail"
	"gitea.mixdep.ru/mix/carrel/internal/ratelimit"
	"gitea.mixdep.ru/mix/carrel/internal/session"
	"gitea.mixdep.ru/mix/carrel/internal/store"
	"gitea.mixdep.ru/mix/carrel/internal/web"
	"gitea.mixdep.ru/mix/carrel/internal/web/handler"
)

var (
	version = "0.10.0"
	commit  = "unknown"
)

// shutdownGrace is how long in-flight requests get to finish before the
// process stops and the keyring is wiped (§18).
const shutdownGrace = 15 * time.Second

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		return runHealthcheck()
	}

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
	// The fan-out registry owns the cross-source polls of §14 and §16. It is
	// created before the session manager so a session that ends can take its
	// polls with it.
	fanouts := fanout.NewRegistry(nil)
	defer fanouts.Close()

	sessions := session.New(session.Options{
		Idle:     settings.SessionIdle(),
		Absolute: settings.SessionAbsolute(),
		OnEnd:    fanouts.CancelSession,
		Cache: session.CacheConfig{
			CollectionTTL:   cfg.Cache.CollectionTTL(),
			MaxCollections:  cfg.Cache.MaxCollections,
			MaxETagEntries:  cfg.Cache.MaxETagEntries,
			MaxBodyBytes:    cfg.Cache.MaxBodyBytes,
			MaxProcessBytes: cfg.Cache.MaxProcessBytes,
			MaxThumbBytes:   cfg.Cache.MaxThumbBytes,
			MaxThumbEntries: cfg.Cache.MaxThumbEntries,
		},
	})

	guard := dav.NewGuard(dav.GuardConfig{
		Allowlist:        cfg.DAV.SSRFAllowlist,
		ConnectTimeout:   cfg.DAV.ConnectTimeout.Duration(),
		RequestTimeout:   cfg.DAV.RequestTimeout.Duration(),
		MaxResponseBytes: cfg.DAV.MaxResponseBytes,
		MaxRedirects:     cfg.DAV.MaxRedirects,
	})

	templates, err := loadTemplates()
	if err != nil {
		logger.Error("templates", "error", err)
		return 1
	}

	loginLimit := ratelimit.New(ratelimit.Options{})
	inviteLimit := ratelimit.New(ratelimit.Options{})
	// The master password is typed rarely and by one person, so there is no
	// reason to forgive a run of wrong ones (§5.4).
	recoveryLimit := ratelimit.New(ratelimit.Options{Free: 1})

	mailQueue := &mail.Queue{
		Store:       st,
		Logger:      logger,
		ServiceName: "Carrel",
	}
	defer mailQueue.Close()

	srv := &handler.Server{
		BasePath:      cfg.BasePath,
		Version:       version,
		Commit:        commit,
		Trust:         trust,
		Sessions:      sessions,
		Store:         st,
		Templates:     templates,
		LoginLimit:    loginLimit,
		InviteLimit:   inviteLimit,
		RecoveryLimit: recoveryLimit,
		Mail:          mailQueue,
		Guard:         guard,
		Photo:         handler.PhotoConfig(cfg.Photo),
		Import:        handler.ImportConfig(cfg.Import),
		Files:         handler.FilesConfig(cfg.Files),
		Fanout:        fanouts,
		Progress:      cfg.Progress,
		Detection:     cfg.Duplicates,
		Logger:        logger,
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
	go sweepLimiter(ctx, recoveryLimit, limiterSweep)
	// A tab closed mid-poll leaves a task nobody will ask about again.
	go sweepFanout(ctx, fanouts, fanoutSweep)

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

// fanoutSweep is how often abandoned fan-out tasks are dropped.
const fanoutSweep = time.Minute

func sweepFanout(ctx context.Context, r *fanout.Registry, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Sweep()
		}
	}
}

// runHealthcheck probes /healthz for container liveness (§18). Distroless
// images have no shell or curl, so Docker invokes the binary directly.
func runHealthcheck() int {
	port := strconv.Itoa(config.DefaultPort)
	if v := strings.TrimSpace(os.Getenv("CARREL_PORT")); v != "" {
		port = v
	}
	path := "/healthz"
	if base := strings.TrimSpace(os.Getenv("CARREL_BASE_PATH")); base != "" {
		path = strings.TrimSuffix(base, "/") + path
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + path)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
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
