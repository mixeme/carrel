package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
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

// S-27.1, S-28: with CARREL_DAVLOOM_PORT the second listener answers its
// own root; when the signal comes both stop and serve returns.
func TestBothListenersStopOnSignal(t *testing.T) {
	free := func() int {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer ln.Close()
		return ln.Addr().(*net.TCPAddr).Port
	}
	web, davloom := free(), free()
	cfg := &conf.Config{Bind: "127.0.0.1", Port: web, DAVloomPort: davloom, DataDir: t.TempDir(), ProgressTotalTimeout: 2 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg) }()

	client := &http.Client{Timeout: time.Second}
	get := func(port int, path string) (*http.Response, error) {
		return client.Get("http://127.0.0.1:" + strconv.Itoa(port) + path)
	}
	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		if resp, err = get(davloom, "/"); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("davloom did not come up: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), `"title":"davloom"`) || resp.Header.Get("Set-Cookie") != "" {
		t.Errorf("davloom root: %s", body)
	}
	if resp, err := get(davloom, "/healthz"); err == nil {
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("the web's routes answer on the davloom port: %d", resp.StatusCode)
		}
		resp.Body.Close()
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not return after the signal")
	}
	if _, err := get(web, "/healthz"); err == nil {
		t.Error("the web port still answers")
	}
}

// S-30.3: a request under way when the signal comes is answered, not cut;
// only then does serve return.
func TestSignalDoesNotCutARequest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	cfg := &conf.Config{Bind: "127.0.0.1", Port: port, DataDir: t.TempDir(), ProgressTotalTimeout: 5 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg) }()

	var conn net.Conn
	for i := 0; i < 50; i++ {
		if conn, err = net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(port)); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	body := "login=x&password=y"
	// Headers and half the body before the signal, the rest after it.
	if _, err := io.WriteString(conn, "POST /setup HTTP/1.1\r\nHost: x\r\nContent-Type: application/x-www-form-urlencoded\r\nContent-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"+body[:5]); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	cancel()
	time.Sleep(300 * time.Millisecond)
	if _, err := io.WriteString(conn, body[5:]); err != nil {
		t.Fatalf("the connection was cut at the signal: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	status := make([]byte, 12)
	if _, err := io.ReadFull(conn, status); err != nil || !strings.HasPrefix(string(status), "HTTP/1.1 ") {
		t.Fatalf("no answer after the signal: %q %v", status, err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not return")
	}
}
