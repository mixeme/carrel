// Command carrel is the Carrel server binary.
package main

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
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

func run(cfg *conf.Config) error {
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

	users := web.NewStubUsers(st)
	srv := web.New(cfg, st, mgr, users, web.JSONRenderer{}, version, commit)

	addr := net.JoinHostPort(cfg.Bind, strconv.Itoa(cfg.Port))
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 20 * time.Second,
	}
	slog.Info("carrel: listening", "addr", addr, "version", version, "commit", commit)
	return httpSrv.ListenAndServe()
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
