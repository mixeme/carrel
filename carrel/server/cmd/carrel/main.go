// Command carrel is the Carrel server binary.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
	// The zone database goes in the binary: the image has no
	// /usr/share/zoneinfo, and the instance zone (C-10) and the zones an
	// event may be moved to (s-agenda-8) must load there too.
	_ "time/tzdata"

	"carrel.dev/carrel/client/document"
	"carrel.dev/carrel/client/renderer"
	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/proxy"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/internal/web"
)

// version and commit are set by -ldflags at build time (C-13: the About
// page must be able to show the running version and commit).
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	cfg, err := conf.Load()
	if err != nil {
		slog.Error("carrel: config", "error", err)
		os.Exit(1)
	}
	if err := setupLog(cfg); err != nil {
		slog.Error("carrel: config", "error", err)
		os.Exit(1)
	}
	if err := run(cfg); err != nil {
		slog.Error("carrel: exiting", "error", err)
		os.Exit(1)
	}
}

func setupLog(cfg *conf.Config) error {
	level, err := cfg.ParseLogLevel()
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
	return nil
}

// run serves until SIGINT or SIGTERM, then shuts down (S-28).
func run(cfg *conf.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serve(ctx, cfg)
}

// serve runs the web listener and, when CARREL_DAVLOOM_PORT is set, the
// davloom one (S-27.1): two http.Servers of one process, each its own mux,
// buckets and log attribute, no cookie of the web on the second. When ctx
// ends both stop taking requests, the ones under way have
// CARREL_PROGRESS_TOTAL_TIMEOUT to finish, and then the keys of every
// session and every opened device are wiped.
func serve(ctx context.Context, cfg *conf.Config) error {
	serverKey, err := crypt.LoadOrCreateServerKey(cfg.DataDir)
	if err != nil {
		return err
	}
	st, err := store.Open(cfg.DataDir, serverKey)
	if err != nil {
		return err
	}
	if st.FirstRun() {
		slog.Info("carrel: first-run mode", "dataDir", cfg.DataDir)
	}
	mgr := session.NewManager(cfg)
	stopPurge := runSessionPurge(mgr, time.Minute)
	defer stopPurge()

	users := user.New(st, mgr, cfg)
	// The client draws the pages that have a screen and answers JSON for the
	// rest (carrel.dev/carrel/client/renderer); the library's files its pages
	// name are served from the binary (C-5.9). This is the one package of the
	// server that imports the client (docs/architecture.md, «Модули»).
	srv := web.New(cfg, st, mgr, users, renderer.New(conf.NormalizeBasePath(cfg.BasePath)), version, commit)
	srv.Static(document.Static, document.Files())

	addr := net.JoinHostPort(cfg.Bind, strconv.Itoa(cfg.Port))
	servers := []*http.Server{{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 20 * time.Second,
	}}
	slog.Info("carrel: listening", "addr", addr, "version", version, "commit", commit)
	var px *proxy.Server
	if cfg.DAVloomPort != 0 {
		px = proxy.New(cfg, users, dav.NewGuard(cfg), version, commit)
		davAddr := net.JoinHostPort(cfg.Bind, strconv.Itoa(cfg.DAVloomPort))
		servers = append(servers, &http.Server{
			Addr:              davAddr,
			Handler:           px.Handler(),
			ReadHeaderTimeout: 20 * time.Second,
		})
		slog.Info("carrel: listening", "addr", davAddr, "listener", "davloom")
	}

	errc := make(chan error, len(servers))
	for _, hs := range servers {
		go func(hs *http.Server) {
			if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errc <- err
			}
		}(hs)
	}
	var runErr error
	select {
	case runErr = <-errc:
	case <-ctx.Done():
		slog.Info("carrel: shutting down")
	}
	wait := cfg.ProgressTotalTimeout
	if wait <= 0 {
		wait = 30 * time.Second
	}
	sctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	var wg sync.WaitGroup
	for _, hs := range servers {
		wg.Add(1)
		go func(hs *http.Server) {
			defer wg.Done()
			if err := hs.Shutdown(sctx); err != nil {
				slog.Warn("carrel: requests cut at shutdown", "addr", hs.Addr, "error", err)
			}
		}(hs)
	}
	wg.Wait()
	// Sessions end: their fanouts are cancelled and their keys wiped (S-4).
	mgr.EndAll()
	if px != nil {
		px.Wipe()
	}
	return runErr
}

func runSessionPurge(mgr *session.Manager, interval time.Duration) func() {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case now := <-ticker.C:
				if n := mgr.PurgeExpired(now); n > 0 {
					slog.Debug("carrel: purged expired sessions", "count", n)
				}
			case <-stop:
				return
			}
		}
	}()
	return func() { close(stop) }
}
