// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GuardConfig tunes outbound DAV connections (§24.2).
type GuardConfig struct {
	Allowlist          []string
	ConnectTimeout     time.Duration
	RequestTimeout     time.Duration
	MaxResponseBytes   int64
	MaxRedirects       int
	InsecureSkipVerify bool // dev/integration only — self-signed TLS
}

// Guard validates outbound URLs and builds an SSRF-safe HTTP client.
type Guard struct {
	allowlist        map[string]struct{}
	connectTimeout   time.Duration
	requestTimeout   time.Duration
	maxResponseBytes int64
	maxRedirects     int
	insecureTLS      bool
	resolver         *net.Resolver
}

// NewGuard returns a guard with defaults applied for zero fields.
func NewGuard(cfg GuardConfig) *Guard {
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 10 * time.Second
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 30 * time.Second
	}
	if cfg.MaxResponseBytes <= 0 {
		cfg.MaxResponseBytes = 10 << 20
	}
	if cfg.MaxRedirects <= 0 {
		cfg.MaxRedirects = 5
	}
	allow := make(map[string]struct{}, len(cfg.Allowlist))
	for _, host := range cfg.Allowlist {
		host = normalizeHost(host)
		if host != "" {
			allow[host] = struct{}{}
		}
	}
	return &Guard{
		allowlist:        allow,
		connectTimeout:   cfg.ConnectTimeout,
		requestTimeout:   cfg.RequestTimeout,
		maxResponseBytes: cfg.MaxResponseBytes,
		maxRedirects:     cfg.MaxRedirects,
		insecureTLS:      cfg.InsecureSkipVerify,
		resolver:         net.DefaultResolver,
	}
}

// HTTPClient returns an http.Client that enforces SSRF rules on every hop.
func (g *Guard) HTTPClient() *http.Client {
	transport := &http.Transport{
		DialContext:           g.dialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: g.insecureTLS}, //nolint:gosec // dev integration flag only
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Transport:     transport,
		Timeout:       g.requestTimeout,
		CheckRedirect: g.checkRedirect,
	}
}

func (g *Guard) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= g.maxRedirects {
		return ErrTooManyRedirects
	}
	if err := g.ValidateURL(req.Context(), req.URL); err != nil {
		return err
	}
	return nil
}

// ValidateURL resolves the host and rejects blocked addresses.
func (g *Guard) ValidateURL(ctx context.Context, u *url.URL) error {
	if u == nil {
		return fmt.Errorf("%w: empty URL", ErrSSRF)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("%w: unsupported scheme %q", ErrSSRF, u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: missing host", ErrSSRF)
	}
	if ip := net.ParseIP(host); ip != nil {
		return g.checkIP(ip, "", g.ipAllowed(ip))
	}
	ips, err := g.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("dav: resolve %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("dav: resolve %q: no addresses", host)
	}
	allowed := g.hostAllowed(host)
	for _, addr := range ips {
		if err := g.checkIP(addr.IP, host, allowed); err == nil {
			return nil
		}
	}
	return fmt.Errorf("%w: host %q resolves only to blocked addresses", ErrSSRF, host)
}

func (g *Guard) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	var (
		targetIP net.IP
	)

	if ip := net.ParseIP(host); ip != nil {
		if err := g.checkIP(ip, "", g.ipAllowed(ip)); err != nil {
			return nil, err
		}
		targetIP = ip
	} else {
		ips, err := g.resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("dav: resolve %q: %w", host, err)
		}
		allowed := g.hostAllowed(host)
		var lastErr error
		for _, addr := range ips {
			if err := g.checkIP(addr.IP, host, allowed); err != nil {
				lastErr = err
				continue
			}
			targetIP = addr.IP
			break
		}
		if targetIP == nil {
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, fmt.Errorf("%w: host %q has no dialable addresses", ErrSSRF, host)
		}
	}

	target := net.JoinHostPort(targetIP.String(), port)
	dialer := &net.Dialer{Timeout: g.connectTimeout}
	return dialer.DialContext(ctx, network, target)
}

func (g *Guard) hostAllowed(host string) bool {
	if len(g.allowlist) == 0 {
		return false
	}
	host = normalizeHost(host)
	if _, ok := g.allowlist[host]; ok {
		return true
	}
	for entry := range g.allowlist {
		if strings.HasSuffix(host, "."+entry) {
			return true
		}
	}
	return false
}

func (g *Guard) ipAllowed(ip net.IP) bool {
	if ip == nil || len(g.allowlist) == 0 {
		return false
	}
	_, ok := g.allowlist[normalizeHost(ip.String())]
	return ok
}

func (g *Guard) checkIP(ip net.IP, host string, allowed ...bool) error {
	isAllowed := len(allowed) > 0 && allowed[0]
	if isAllowed {
		return nil
	}
	if ip == nil || ip.IsUnspecified() {
		return fmt.Errorf("%w: unspecified address for %q", ErrSSRF, host)
	}
	if ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("%w: blocked address %s for %q", ErrSSRF, ip, host)
	}
	if ip.IsPrivate() {
		return fmt.Errorf("%w: private address %s for %q", ErrSSRF, ip, host)
	}
	for _, block := range blockedNets {
		if block.Contains(ip) {
			return fmt.Errorf("%w: blocked address %s for %q", ErrSSRF, ip, host)
		}
	}
	return nil
}

var blockedNets []*net.IPNet

func init() {
	for _, cidr := range []string{
		"0.0.0.0/8",
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	} {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(err)
		}
		blockedNets = append(blockedNets, n)
	}
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
}

// limitedBody wraps a response body with a size cap.
func limitedBody(r io.ReadCloser, max int64) io.ReadCloser {
	return struct {
		io.Reader
		io.Closer
	}{
		Reader: io.LimitReader(r, max+1),
		Closer: r,
	}
}

// readLimited reads up to max+1 bytes and returns ErrResponseTooLarge when exceeded.
func readLimited(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		return nil, errors.New("dav: max response size must be positive")
	}
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, ErrResponseTooLarge
	}
	return data, nil
}
