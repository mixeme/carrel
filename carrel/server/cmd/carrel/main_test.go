package main

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
)

// TestEmptyVolumeStarts confirms the server comes up and answers /healthz
// with no state file yet — first-run mode via internal/store.
func TestEmptyVolumeStarts(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := ln.Addr().String()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}

	portN, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	cfg := &conf.Config{Bind: host, Port: portN, DataDir: t.TempDir()}

	errc := make(chan error, 1)
	go func() { errc <- run(cfg) }()

	client := &http.Client{Timeout: time.Second}
	var resp *http.Response
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err = client.Get("http://" + addr + "/healthz")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server never came up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("healthz = %d %q, want 200 \"ok\"", resp.StatusCode, body)
	}

	select {
	case err := <-errc:
		t.Fatalf("server exited early: %v", err)
	default:
	}
}
