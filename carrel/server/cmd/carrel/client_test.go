package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
)

// The binary answers a page that has a screen with the client's HTML, and
// serves the library's files that page names from itself, on this origin
// (C-5.9), under the headers of every answer (C-11): the policy of content
// as it is. Everything else stays JSON until its screen is built — the
// client's own checks hold that (carrel/client/guard, renderer_test.go).
func TestPagesAreTheClients(t *testing.T) {
	for _, base := range []string{"", "/carrel"} {
		t.Run("base "+strconv.Quote(base), func(t *testing.T) {
			origin := startServer(t, base)
			client := &http.Client{Timeout: 10 * time.Second}
			get := func(path string) (*http.Response, string) {
				t.Helper()
				resp, err := client.Get(origin + path)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}
				return resp, string(body)
			}
			secured := func(what string, resp *http.Response) {
				t.Helper()
				for name, want := range map[string]string{
					"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'",
					"X-Content-Type-Options":  "nosniff",
					"X-Frame-Options":         "DENY",
					"Referrer-Policy":         "same-origin",
				} {
					if got := resp.Header.Get(name); got != want {
						t.Errorf("%s: %s %q, want %q", what, name, got, want)
					}
				}
			}

			// An empty volume: /login leads to the first run's /setup.
			resp, page := get(base + "/login")
			if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/html; charset=utf-8" {
				t.Fatalf("/login: %d %q\n%.300s", resp.StatusCode, resp.Header.Get("Content-Type"), page)
			}
			if !strings.HasPrefix(page, "<!doctype html>") || !strings.HasSuffix(page, "</html>") {
				t.Errorf("/login: not a document: %.200s", page)
			}
			secured("/login", resp)
			if got := resp.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("/login: Cache-Control %q, want no-store", got)
			}

			files := regexp.MustCompile(`(?:src|href)="(`+regexp.QuoteMeta(base)+`/static/tc/[0-9a-f]{12}/[^"]+)"`).FindAllStringSubmatch(page, -1)
			if len(files) != 4 {
				t.Fatalf("/login names %d files of the library, want 4 (theme, stylesheet, script, live layer): %v", len(files), files)
			}
			kinds := map[string]string{".css": "text/css; charset=utf-8", ".js": "application/javascript; charset=utf-8"}
			var css, cssPath string
			for _, m := range files {
				path := m[1]
				resp, body := get(path)
				want := kinds[path[strings.LastIndex(path, "."):]]
				if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != want || body == "" {
					t.Errorf("%s: %d %q, %d bytes", path, resp.StatusCode, resp.Header.Get("Content-Type"), len(body))
				}
				secured(path, resp)
				if got := resp.Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
					t.Errorf("%s: Cache-Control %q", path, got)
				}
				if strings.HasSuffix(path, ".css") {
					css, cssPath = body, path
				}
			}
			// The fonts the stylesheet names, beside it.
			fonts := regexp.MustCompile(`url\(["']?\.\./fonts/([^)"']+)["']?\)`).FindAllStringSubmatch(css, -1)
			if len(fonts) == 0 {
				t.Fatal("the stylesheet names no font")
			}
			font := strings.TrimSuffix(cssPath, "css/typecase.css") + "fonts/" + fonts[0][1]
			resp, body := get(font)
			if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "font/") || body == "" {
				t.Errorf("%s: %d %q, %d bytes", font, resp.StatusCode, resp.Header.Get("Content-Type"), len(body))
			}
			secured(font, resp)

			// Another version's file is not found, and is not kept.
			stale := regexp.MustCompile(`/static/tc/[0-9a-f]{12}/`).ReplaceAllString(cssPath, "/static/tc/000000000000/")
			if resp, _ := get(stale); resp.StatusCode != http.StatusNotFound || resp.Header.Get("Cache-Control") != "no-store" {
				t.Errorf("%s: %d %q, want 404 no-store", stale, resp.StatusCode, resp.Header.Get("Cache-Control"))
			}
			// Nothing but reading: the files take no POST.
			resp2, err := client.Post(origin+cssPath, "text/plain", strings.NewReader("x"))
			if err != nil {
				t.Fatal(err)
			}
			resp2.Body.Close()
			if resp2.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("POST %s: %d, want 405", cssPath, resp2.StatusCode)
			}
		})
	}
}

// startServer runs the binary's serve on a free port over an empty volume,
// under base, until the test ends; it answers at the origin it returns.
func startServer(t *testing.T, base string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	cfg := &conf.Config{Bind: "127.0.0.1", Port: port, DataDir: t.TempDir(), BasePath: base, ProgressTotalTimeout: 2 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("serve: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("serve did not return")
		}
	})
	origin := "http://127.0.0.1:" + strconv.Itoa(port)
	for i := 0; i < 100; i++ {
		if resp, err := http.Get(origin + base + "/healthz"); err == nil {
			resp.Body.Close()
			return origin
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the server did not come up")
	return ""
}
