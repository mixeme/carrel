package main

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestEmptyVolumeStarts confirms the server comes up and answers /healthz
// with no state on disk at all — the plan's first-run mode. There is no
// store yet to point at an empty CARREL_DATA_DIR; this test exists so the
// check has somewhere to grow into as internal/store lands.
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

	errc := make(chan error, 1)
	go func() { errc <- run(host, port) }()

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
