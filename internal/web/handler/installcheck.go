// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/config"
	"gitea.mixdep.ru/mix/carrel/internal/installcheck"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

const (
	fieldPublicURL     = "public_url"
	installProbeTTL    = 2 * time.Minute
	installUploadLimit = 20<<20 + 1
)

func (g *InstallCheckGate) register() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.probe = &installProbeState{token: token, expires: time.Now().Add(installProbeTTL)}
	return token, nil
}

func (g *InstallCheckGate) valid(token string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	p := g.probe
	return p != nil && p.token == token && time.Now().Before(p.expires)
}

func (g *InstallCheckGate) clear() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.probe = nil
}

func (s *Server) InstallCheckEcho(w http.ResponseWriter, r *http.Request) {
	if !s.InstallCheck.valid(r.PathValue("token")) {
		http.NotFound(w, r)
		return
	}
	echo := installcheck.Echo{
		Host:           r.Host,
		ForwardedHost:  r.Header.Get("X-Forwarded-Host"),
		ForwardedProto: r.Header.Get("X-Forwarded-Proto"),
		ClientIP:       ClientIP(r),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(echo)
}

func (s *Server) InstallCheckSSE(w http.ResponseWriter, r *http.Request) {
	if !s.InstallCheck.valid(r.PathValue("token")) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	_ = writeSSERaw(w, "ready", "ok")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) InstallCheckUpload(w http.ResponseWriter, r *http.Request) {
	if !s.InstallCheck.valid(r.PathValue("token")) {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	_, _ = io.Copy(io.Discard, r.Body)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminRunInstallCheck(r *http.Request, actor store.Actor) (adminView, error) {
	publicURL := strings.TrimSpace(r.PostFormValue(fieldPublicURL))
	if publicURL == "" {
		return adminView{}, fmt.Errorf("enter the public address Carrel is reached at")
	}
	if _, err := url.Parse(publicURL); err != nil {
		return adminView{}, fmt.Errorf("public address is not a valid URL")
	}
	if s.DataDir != "" {
		if err := config.SetPublicURL(s.DataDir, publicURL); err != nil {
			return adminView{}, err
		}
		s.PublicURL = publicURL
	}

	token, err := s.InstallCheck.register()
	if err != nil {
		return adminView{}, err
	}
	defer s.InstallCheck.clear()

	probes := s.installProbes(publicURL, token)
	client := &http.Client{Timeout: 45 * time.Second}

	cfg := installcheck.Config{
		PublicURL:       publicURL,
		BasePath:        s.BasePath,
		DataDir:         s.DataDir,
		MaxUploadBytes:  s.filesMaxUpload(),
		FanoutTimeout:   s.fanoutTotalTimeout(),
		LocalOnly:       s.bindLocalOnly(),
		HasTrustedProxy: len(s.TrustedProxies) > 0,
	}
	result := installcheck.Run(r.Context(), cfg, probes, client)

	detail := fmt.Sprintf("%d ok · %d failed · %d warnings", result.OK, result.Fail, result.Warn)
	_ = s.Store.Log(store.AuditEntry{
		Action:     store.ActionInstallCheck,
		ActorID:    actor.ID,
		ActorLogin: actor.Login,
		IP:         actor.IP,
		Detail:     detail,
	})

	return adminView{
		PublicURL:     publicURL,
		InstallResult: &result,
		LastInstallAt: time.Now(),
	}, nil
}

func (s *Server) installProbes(publicURL, token string) installcheck.Probes {
	join := func(path string) string {
		return strings.TrimSuffix(strings.TrimSpace(publicURL), "/") + s.Path(path)
	}
	return installcheck.Probes{
		Health: join("/healthz"),
		Echo:   join("/installcheck/" + token + "/echo"),
		SSE:    join("/installcheck/" + token + "/sse"),
		Upload: join("/installcheck/" + token + "/upload"),
		Login:  join("/login"),
	}
}

func (s *Server) bindLocalOnly() bool {
	bind := strings.TrimSpace(s.Bind)
	return bind == "127.0.0.1" || bind == "localhost" || bind == "::1"
}

func (s *Server) fanoutTotalTimeout() time.Duration {
	if s.Progress.TotalTimeout.Duration() > 0 {
		return s.Progress.TotalTimeout.Duration()
	}
	return 30 * time.Second
}

func (s *Server) defaultPublicURL(r *http.Request) string {
	if u := strings.TrimSpace(s.PublicURL); u != "" {
		return u
	}
	return s.publicURL(r, "")
}
