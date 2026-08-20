// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package installcheck_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/installcheck"
)

func TestRunAgainstLocalServer(t *testing.T) {
	token := "testtoken"
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/installcheck/"+token+"/echo", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(installcheck.Echo{
			Host: r.Host, ClientIP: r.RemoteAddr,
		})
	})
	mux.HandleFunc("/installcheck/"+token+"/sse", func(w http.ResponseWriter, wreq *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: ready\ndata: ok\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})
	mux.HandleFunc("/installcheck/"+token+"/upload", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "carrel_csrf", Value: "x", HttpOnly: true, SameSite: http.SameSiteLaxMode})
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	probes := installcheck.Probes{
		Health: srv.URL + "/healthz",
		Echo:   srv.URL + "/installcheck/" + token + "/echo",
		SSE:    srv.URL + "/installcheck/" + token + "/sse",
		Upload: srv.URL + "/installcheck/" + token + "/upload",
		Login:  srv.URL + "/login",
	}
	cfg := installcheck.Config{
		PublicURL:        srv.URL,
		DataDir:          t.TempDir(),
		MaxUploadBytes:   256 << 20,
		FanoutTimeout:    30 * time.Second,
		ProbeUploadBytes: 4096,
	}
	res := installcheck.Run(context.Background(), cfg, probes, srv.Client())
	for _, row := range res.Rows {
		if row.Status == installcheck.StatusFail {
			t.Errorf("%s: %s", row.Name, row.Detail)
		}
	}
	foundSSE := false
	for _, row := range res.Rows {
		if row.Name == "Event stream (SSE)" && row.Status == installcheck.StatusOK {
			foundSSE = true
		}
	}
	if !foundSSE {
		t.Fatal("SSE check did not pass")
	}
}

func TestParsePublicURLRequiresHost(t *testing.T) {
	t.Parallel()
	cfg := installcheck.Config{PublicURL: "https://", DataDir: t.TempDir()}
	res := installcheck.Run(context.Background(), cfg, installcheck.Probes{}, nil)
	if len(res.Rows) == 0 || res.Rows[0].Status != installcheck.StatusFail {
		t.Fatalf("got %#v", res.Rows)
	}
}
