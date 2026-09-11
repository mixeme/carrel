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

	"carrel.dev/carrel/server/internal/conf"
)

// ErrSSRF is returned when a target address is not permitted.
var ErrSSRF = errors.New("dav: outbound address blocked")

// Guard validates outbound DAV targets and builds guarded HTTP clients (S-5.1).
type Guard struct {
	allow       map[string]struct{}
	connect     time.Duration
	request     time.Duration
	maxBody     int64
	maxRedirect int
	resolver    *net.Resolver
}

// NewGuard builds a guard from C-12 DAV settings.
func NewGuard(c *conf.Config) *Guard {
	allow := make(map[string]struct{})
	for _, h := range strings.Split(c.DAVSSRFAllowlist, ",") {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" {
			allow[h] = struct{}{}
		}
	}
	return &Guard{
		allow:       allow,
		connect:     c.DAVConnectTimeout,
		request:     c.DAVRequestTimeout,
		maxBody:     c.DAVMaxResponseBytes,
		maxRedirect: c.DAVMaxRedirects,
		resolver:    net.DefaultResolver,
	}
}

// HTTPClient returns an HTTP client that enforces SSRF rules on every request.
func (g *Guard) HTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           g.dialContext,
		DialTLSContext:        g.dialTLSContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Transport: &limitTransport{base: transport, max: g.maxBody},
		Timeout:   g.request,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= g.maxRedirect {
				return fmt.Errorf("dav: too many redirects")
			}
			return nil
		},
	}
}

func (g *Guard) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	allow := g.allowlisted(host)

	ips, err := g.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("dav: no addresses for %s", host)
	}

	dialer := net.Dialer{Timeout: g.connect}
	for _, ip := range ips {
		if allow || !blockedIP(ip.IP) {
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrSSRF, host)
}

func (g *Guard) dialTLSContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	conn, err := g.dialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	cfg := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	return tls.Client(conn, cfg), nil
}

func (g *Guard) allowlisted(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	_, ok := g.allow[host]
	return ok
}

func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		return privateIPv4(ip4)
	}
	if len(ip) == net.IPv6len && ip.To4() == nil {
		return privateIPv6(ip)
	}
	return false
}

func privateIPv4(ip net.IP) bool {
	switch {
	case ip[0] == 10:
		return true
	case ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31:
		return true
	case ip[0] == 192 && ip[1] == 168:
		return true
	case ip[0] == 127:
		return true
	case ip[0] == 169 && ip[1] == 254:
		return true
	case ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127: // CGNAT
		return true
	default:
		return false
	}
}

func privateIPv6(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	// Unique local fc00::/7
	return ip[0]&0xfe == 0xfc
}

type limitTransport struct {
	base http.RoundTripper
	max  int64
}

func (t *limitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.Body != nil && t.max > 0 {
		resp.Body = &limitedBody{ReadCloser: resp.Body, remain: t.max}
	}
	return resp, nil
}

type limitedBody struct {
	io.ReadCloser
	remain int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.remain <= 0 {
		return 0, fmt.Errorf("dav: response exceeds limit")
	}
	if int64(len(p)) > b.remain {
		p = p[:b.remain]
	}
	n, err := b.ReadCloser.Read(p)
	b.remain -= int64(n)
	return n, err
}

// ResolveWellKnown follows a well-known path from base and returns the final URL.
func ResolveWellKnown(ctx context.Context, client *http.Client, base, wellKnown string) (string, string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", "", err
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + wellKnown
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	detail := fmt.Sprintf("GET %s → %d", wellKnown, resp.StatusCode)
	if resp.StatusCode/100 == 2 || resp.StatusCode == http.StatusMovedPermanently ||
		resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect ||
		resp.StatusCode == http.StatusPermanentRedirect {
		final := resp.Request.URL.String()
		if loc := resp.Header.Get("Location"); loc != "" && resp.StatusCode/100 == 3 {
			if lu, err := url.Parse(loc); err == nil {
				final = resp.Request.URL.ResolveReference(lu).String()
			}
		}
		return final, detail, nil
	}
	return "", detail, fmt.Errorf("well-known %s: HTTP %d", wellKnown, resp.StatusCode)
}
