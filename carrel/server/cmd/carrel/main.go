// Command carrel is the Carrel server binary. It is still a skeleton: it
// listens and answers /healthz, and nothing else yet.
package main

import (
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"

	"carrel.dev/carrel/server/internal/conf"
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
	if err := run(cfg); err != nil {
		slog.Error("carrel: exiting", "error", err)
		os.Exit(1)
	}
}

func run(cfg *conf.Config) error {
	addr := net.JoinHostPort(cfg.Bind, strconv.Itoa(cfg.Port))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)

	srv := &http.Server{Addr: addr, Handler: mux}
	slog.Info("carrel: listening", "addr", addr, "version", version, "commit", commit)
	return srv.ListenAndServe()
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}
