// Command carrel is the Carrel server binary. It is still a skeleton: it
// listens and answers /healthz, and nothing else yet.
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
	_ = session.NewManager(cfg)

	addr := net.JoinHostPort(cfg.Bind, strconv.Itoa(cfg.Port))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
		// A request that never finishes its headers must not hold a
		// connection open forever.
		ReadHeaderTimeout: 20 * time.Second,
	}
	slog.Info("carrel: listening", "addr", addr, "version", version, "commit", commit)
	return srv.ListenAndServe()
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}
