// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package installcheck runs the administration installation check of §18.1:
// Carrel calls itself at the declared public address and reports what a browser
// or DAV client would see through the reverse proxy.
package installcheck

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// probeUploadSize is large enough to trip a typical 1–20 MiB proxy limit
	// while staying below Carrel's own file upload ceiling (256 MiB).
	probeUploadSize = 20<<20 + 1

	sseSlowThreshold = 2 * time.Second
)

// Status is how a row should read on the results table.
type Status string

const (
	StatusOK    Status = "ok"
	StatusFail  Status = "fail"
	StatusWarn  Status = "warn"
	StatusLocal Status = "local"
)

// Row is one line of the installation check.
type Row struct {
	Name   string
	Detail string
	Advice string
	Status Status
}

// Result is the full report shown on /admin/install.
type Result struct {
	Rows     []Row
	Duration time.Duration
	OK       int
	Fail     int
	Warn     int
}

// Echo is what the probe handler saw on the request that reached Carrel.
type Echo struct {
	Host           string `json:"host"`
	ForwardedHost  string `json:"forwarded_host"`
	ForwardedProto string `json:"forwarded_proto"`
	ClientIP       string `json:"client_ip"`
}

// Config is everything the check needs besides probe URLs.
type Config struct {
	PublicURL       string
	BasePath        string
	DataDir         string
	MaxUploadBytes  int64
	FanoutTimeout   time.Duration
	LocalOnly       bool
	HasTrustedProxy bool
	// ProbeUploadBytes overrides the upload probe size; zero keeps the default.
	ProbeUploadBytes int64
}

// Probes are the absolute URLs of the short-lived probe endpoints.
type Probes struct {
	Health string
	Echo   string
	SSE    string
	Upload string
	Login  string
}

// Run executes every check against the public base URL. ctx cancels in-flight
// HTTP requests; probes must already be registered on the server.
func Run(ctx context.Context, cfg Config, probes Probes, client *http.Client) Result {
	start := time.Now()
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	if client.Timeout == 0 {
		client.Timeout = 45 * time.Second
	}

	var rows []Row
	add := func(name, detail, advice string, st Status) {
		rows = append(rows, Row{Name: name, Detail: detail, Advice: advice, Status: st})
	}

	public, err := parsePublicURL(cfg.PublicURL, cfg.BasePath)
	if err != nil {
		add("Public address", err.Error(), "Enter the base URL people use in a browser, including https://", StatusFail)
		return summarize(rows, time.Since(start))
	}

	if cfg.LocalOnly || isLoopbackHost(public.Hostname()) {
		add("Reachable from outside", "This instance listens on loopback only — nothing on the network can reach it",
			"Bind to a network interface or put a reverse proxy in front before expecting remote clients", StatusLocal)
	} else {
		rows = append(rows, checkReachable(ctx, client, probes.Health)...)
	}

	if public.Scheme == "https" {
		rows = append(rows, checkCertificate(public)...)
	} else {
		add("Certificate", "Public address uses http — TLS terminates elsewhere or not at all",
			"Use https:// in the public address when the proxy terminates TLS", StatusWarn)
	}

	echo, echoErr := fetchEcho(ctx, client, probes.Echo)
	if echoErr != nil {
		add("Proxy headers", echoErr.Error(),
			"Ensure the reverse proxy forwards Host, X-Forwarded-Host, X-Forwarded-Proto and X-Forwarded-For", StatusFail)
	} else {
		rows = append(rows, checkBaseAddress(public, echo)...)
		rows = append(rows, checkScheme(public, echo, cfg.HasTrustedProxy)...)
		rows = append(rows, checkClientAddress(echo, cfg.HasTrustedProxy)...)
	}

	rows = append(rows, checkSSE(ctx, client, probes.SSE)...)
	rows = append(rows, checkUpload(ctx, client, probes.Upload, cfg.MaxUploadBytes, cfg.ProbeUploadBytes)...)
	rows = append(rows, checkTimeout(cfg.FanoutTimeout)...)
	rows = append(rows, checkCookie(ctx, client, probes.Login, public.Scheme == "https")...)

	rows = append(rows, checkClock()...)
	rows = append(rows, checkDataDir(cfg.DataDir)...)

	return summarize(rows, time.Since(start))
}

func summarize(rows []Row, d time.Duration) Result {
	var ok, fail, warn int
	for _, row := range rows {
		switch row.Status {
		case StatusOK:
			ok++
		case StatusFail:
			fail++
		case StatusWarn, StatusLocal:
			warn++
		}
	}
	return Result{Rows: rows, Duration: d, OK: ok, Fail: fail, Warn: warn}
}

func parsePublicURL(raw, basePath string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("no public address configured")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid public address: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("public address must start with http:// or https://")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("public address is missing a host name")
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	if basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	if basePath != "" && basePath != "/" {
		if u.Path == "" {
			u.Path = basePath
		} else if !strings.HasPrefix(u.Path, basePath) {
			u.Path = strings.TrimSuffix(basePath, "/") + u.Path
		}
	}
	return u, nil
}

func isLoopbackHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(host)
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func checkReachable(ctx context.Context, client *http.Client, healthURL string) []Row {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return []Row{{Name: "Reachable from outside", Detail: err.Error(), Status: StatusFail}}
	}
	t0 := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(t0)
	if err != nil {
		return []Row{{Name: "Reachable from outside", Detail: err.Error(),
			Advice: "Check DNS, the proxy, and that Carrel is running", Status: StatusFail}}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return []Row{{Name: "Reachable from outside",
			Detail: fmt.Sprintf("GET %s → %d · %d ms", healthURL, resp.StatusCode, latency.Milliseconds()),
			Advice: "The public address must reach Carrel's /healthz with 200", Status: StatusFail}}
	}
	return []Row{{Name: "Reachable from outside",
		Detail: fmt.Sprintf("GET %s → 200 · %d ms", healthURL, latency.Milliseconds()), Status: StatusOK}}
}

func checkCertificate(public *url.URL) []Row {
	host := public.Hostname()
	if host == "" {
		return []Row{{Name: "Certificate", Detail: "no host to inspect", Status: StatusFail}}
	}
	addr := host
	if !strings.Contains(addr, ":") {
		addr = addr + ":443"
	}
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return []Row{{Name: "Certificate", Detail: err.Error(),
			Advice: "Fix TLS on the proxy or the certificate chain", Status: StatusFail}}
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return []Row{{Name: "Certificate", Detail: "no certificate presented", Status: StatusFail}}
	}
	leaf := certs[0]
	nameOK := leaf.VerifyHostname(host) == nil
	issuer := leaf.Issuer.CommonName
	if len(leaf.Issuer.Organization) > 0 {
		issuer = leaf.Issuer.Organization[0]
	}
	detail := fmt.Sprintf("%s · until %s · chain %d deep", issuer, leaf.NotAfter.Format("2006-01-02"), len(certs))
	if nameOK {
		detail += " · name matches"
		return []Row{{Name: "Certificate", Detail: detail, Status: StatusOK}}
	}
	return []Row{{Name: "Certificate", Detail: detail + " · name mismatch",
		Advice: "The certificate must match the public host name", Status: StatusFail}}
}

func fetchEcho(ctx context.Context, client *http.Client, echoURL string) (*Echo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, echoURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("echo probe → %d", resp.StatusCode)
	}
	var echo Echo
	if err := jsonDecode(resp.Body, &echo); err != nil {
		return nil, err
	}
	return &echo, nil
}

func checkBaseAddress(public *url.URL, echo *Echo) []Row {
	want := strings.ToLower(public.Hostname())
	got := hostOnly(echo.Host)
	fwd := hostOnly(echo.ForwardedHost)
	if fwd == "" {
		fwd = got
	}
	if got == want && (echo.ForwardedHost == "" || fwd == want) {
		return []Row{{Name: "Base address",
			Detail: "Host and X-Forwarded-Host match the public address", Status: StatusOK}}
	}
	return []Row{{Name: "Base address",
		Detail: fmt.Sprintf("Host %q · X-Forwarded-Host %q · want %q", echo.Host, echo.ForwardedHost, want),
		Advice: "Set proxy Host / X-Forwarded-Host to the public name and base_path in config if mounted in a subfolder",
		Status: StatusFail}}
}

func hostOnly(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return strings.ToLower(h)
	}
	return strings.ToLower(host)
}

func checkScheme(public *url.URL, echo *Echo, trusted bool) []Row {
	proto := strings.ToLower(strings.TrimSpace(strings.Split(echo.ForwardedProto, ",")[0]))
	if proto == "" && public.Scheme == "https" {
		if !trusted {
			return []Row{{Name: "Scheme behind the proxy",
				Detail: "No trusted proxy configured — X-Forwarded-Proto is ignored",
				Advice: "Add the proxy address to trusted_proxies so Secure cookies work", Status: StatusFail}}
		}
		return []Row{{Name: "Scheme behind the proxy",
			Detail: "X-Forwarded-Proto not seen — links follow the public https address",
			Status: StatusWarn}}
	}
	if public.Scheme == "https" && proto == "https" {
		return []Row{{Name: "Scheme behind the proxy",
			Detail: "X-Forwarded-Proto: https · links are built as https", Status: StatusOK}}
	}
	if public.Scheme == "http" && proto == "" {
		return []Row{{Name: "Scheme behind the proxy",
			Detail: "Public address is http", Status: StatusOK}}
	}
	return []Row{{Name: "Scheme behind the proxy",
		Detail: fmt.Sprintf("X-Forwarded-Proto: %s · public address is %s", proto, public.Scheme),
		Advice: "Set X-Forwarded-Proto with set, not setifempty (see README)", Status: StatusFail}}
}

func checkClientAddress(echo *Echo, trusted bool) []Row {
	if !trusted {
		return []Row{{Name: "Client address",
			Detail: "No trusted proxy — rate limits use the direct connection only",
			Advice: "Add the proxy to trusted_proxies so X-Forwarded-For is honoured", Status: StatusWarn}}
	}
	if echo.ClientIP == "" {
		return []Row{{Name: "Client address", Detail: "Could not determine a client address", Status: StatusFail}}
	}
	return []Row{{Name: "Client address",
		Detail: fmt.Sprintf("X-Forwarded-For seen · limits count %s, not the proxy", echo.ClientIP), Status: StatusOK}}
}

func checkSSE(ctx context.Context, client *http.Client, sseURL string) []Row {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseURL, nil)
	if err != nil {
		return []Row{{Name: "Event stream (SSE)", Detail: err.Error(), Status: StatusFail}}
	}
	req.Header.Set("Accept", "text/event-stream")
	t0 := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return []Row{{Name: "Event stream (SSE)", Detail: err.Error(),
			Advice: "nginx: proxy_buffering off for fan-out streams; Apache: flushpackets=on", Status: StatusFail}}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return []Row{{Name: "Event stream (SSE)",
			Detail: fmt.Sprintf("GET stream → %d", resp.StatusCode),
			Advice: "Allow GET on /app/find/*/stream through the proxy without auth stripping", Status: StatusFail}}
	}
	first, err := firstSSEDataLine(resp.Body, 35*time.Second)
	delay := time.Since(t0)
	if err != nil {
		return []Row{{Name: "Event stream (SSE)", Detail: err.Error(),
			Advice: "nginx: proxy_buffering off for /app/find/*/stream", Status: StatusFail}}
	}
	_ = first
	if delay > sseSlowThreshold {
		return []Row{{Name: "Event stream (SSE)",
			Detail: fmt.Sprintf("first event after %.1f s instead of ~0.2 s — the response is buffered", delay.Seconds()),
			Advice: "nginx: proxy_buffering off for /app/find/*/stream", Status: StatusFail}}
	}
	return []Row{{Name: "Event stream (SSE)",
		Detail: fmt.Sprintf("first event after %.1f s", delay.Seconds()), Status: StatusOK}}
}

func firstSSEDataLine(r io.Reader, limit time.Duration) (string, error) {
	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "data:") {
				ch <- result{strings.TrimSpace(strings.TrimPrefix(line, "data:")), nil}
				return
			}
		}
		if err := sc.Err(); err != nil {
			ch <- result{"", err}
			return
		}
		ch <- result{"", fmt.Errorf("stream ended before the first event")}
	}()
	select {
	case res := <-ch:
		return res.text, res.err
	case <-time.After(limit):
		return "", fmt.Errorf("no event within %.0f s", limit.Seconds())
	}
}

func checkUpload(ctx context.Context, client *http.Client, uploadURL string, carrelMax, probeSize int64) []Row {
	if carrelMax <= 0 {
		carrelMax = 256 << 20
	}
	if probeSize <= 0 {
		probeSize = probeUploadSize
	}
	body := io.LimitReader(&zeroReader{}, probeSize)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, body)
	if err != nil {
		return []Row{{Name: "Upload size", Detail: err.Error(), Status: StatusFail}}
	}
	req.ContentLength = probeSize
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		return []Row{{Name: "Upload size", Detail: err.Error(), Status: StatusFail}}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	sizeMB := probeSize >> 20
	if sizeMB == 0 {
		sizeMB = 1
	}
	carrelMB := carrelMax >> 20
	switch resp.StatusCode {
	case http.StatusRequestEntityTooLarge, http.StatusBadRequest:
		return []Row{{Name: "Upload size",
			Detail: fmt.Sprintf("%d MiB probe → %d from the proxy; Carrel allows %d MiB", sizeMB, resp.StatusCode, carrelMB),
			Advice: "Raise client_max_body_size / LimitRequestBody above Carrel's upload ceiling", Status: StatusFail}}
	case http.StatusOK, http.StatusNoContent:
		return []Row{{Name: "Upload size",
			Detail: fmt.Sprintf("%d MiB probe accepted · Carrel allows %d MiB", sizeMB, carrelMB), Status: StatusOK}}
	default:
		return []Row{{Name: "Upload size",
			Detail: fmt.Sprintf("%d MiB probe → %d", sizeMB, resp.StatusCode),
			Advice: "Ensure large uploads reach Carrel, not only small forms", Status: StatusWarn}}
	}
}

func checkTimeout(fanout time.Duration) []Row {
	if fanout <= 0 {
		fanout = 30 * time.Second
	}
	rec := fanout + 30*time.Second
	if rec < 60*time.Second {
		rec = 60 * time.Second
	}
	return []Row{{Name: "Long request timeout",
		Detail: fmt.Sprintf("fan-out ceiling is %.0f s; proxy timeout cannot be measured from here", fanout.Seconds()),
		Advice: fmt.Sprintf("proxy_read_timeout ≥ %.0fs", rec.Seconds()), Status: StatusWarn}}
}

func checkCookie(ctx context.Context, client *http.Client, loginURL string, wantSecure bool) []Row {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loginURL, nil)
	if err != nil {
		return []Row{{Name: "Cookie", Detail: err.Error(), Status: StatusFail}}
	}
	resp, err := client.Do(req)
	if err != nil {
		return []Row{{Name: "Cookie", Detail: err.Error(), Status: StatusFail}}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	var csrf *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "carrel_csrf" {
			csrf = c
		}
	}
	if csrf == nil {
		return []Row{{Name: "Cookie", Detail: "CSRF cookie not set on login page", Status: StatusFail}}
	}
	parts := []string{}
	if csrf.HttpOnly {
		parts = append(parts, "HttpOnly")
	}
	if csrf.Secure {
		parts = append(parts, "Secure")
	}
	if csrf.SameSite == http.SameSiteLaxMode {
		parts = append(parts, "SameSite=Lax")
	} else if csrf.SameSite == http.SameSiteStrictMode {
		parts = append(parts, "SameSite=Strict")
	}
	detail := strings.Join(parts, ", ")
	detail += " on CSRF cookie"
	if !csrf.HttpOnly {
		return []Row{{Name: "Cookie", Detail: detail, Advice: "Session cookies must be HttpOnly", Status: StatusFail}}
	}
	if wantSecure && !csrf.Secure {
		return []Row{{Name: "Cookie", Detail: detail + " · Secure missing",
			Advice: "Terminate TLS at the proxy and set trusted_proxies so Secure cookies work", Status: StatusFail}}
	}
	return []Row{{Name: "Cookie", Detail: detail, Status: StatusOK}}
}

func checkClock() []Row {
	now := time.Now()
	loc := now.Location()
	name := loc.String()
	return []Row{{Name: "Clock",
		Detail: fmt.Sprintf("local time %s · TZ %s", now.Format("2006-01-02 15:04:05 MST"), name),
		Status: StatusOK}}
}

func checkDataDir(dir string) []Row {
	if strings.TrimSpace(dir) == "" {
		return []Row{{Name: "Data folder", Detail: "data directory not configured", Status: StatusFail}}
	}
	free, err := diskFree(dir)
	if err != nil {
		return []Row{{Name: "Data folder", Detail: err.Error(), Status: StatusFail}}
	}
	if err := dirWritable(dir); err != nil {
		return []Row{{Name: "Data folder",
			Detail: fmt.Sprintf("%s · not writable: %v", dir, err),
			Advice: "Fix permissions on the data volume", Status: StatusFail}}
	}
	return []Row{{Name: "Data folder",
		Detail: fmt.Sprintf("%s · writable · %s free", dir, formatBytes(free)), Status: StatusOK}}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func formatBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.0f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
