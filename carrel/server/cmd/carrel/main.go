// Command carrel is the Carrel server binary. It is still a skeleton: it
// listens and answers /healthz, and nothing else yet.
package main

import (
	"log/slog"
	"net"
	"net/http"
	"os"
)

// version and commit are set by -ldflags at build time (C-13: the About
// page must be able to show the running version and commit).
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	if err := run(os.Getenv("CARREL_BIND"), os.Getenv("CARREL_PORT")); err != nil {
		slog.Error("carrel: exiting", "error", err)
		os.Exit(1)
	}
}

func run(bind, port string) error {
	if port == "" {
		port = "8080"
	}
	addr := net.JoinHostPort(bind, port)

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
