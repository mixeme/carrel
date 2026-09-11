package dav

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
	"unicode"

	"carrel.dev/carrel/server/internal/conf"
)

// ErrSSRF is returned when a target address is not permitted.
var ErrSSRF = errors.New("dav: outbound address blocked")

// ErrResponseTooLarge is returned when a reply exceeds
// CARREL_DAV_MAX_RESPONSE_BYTES. It is a state on the screen, not a 500.
var ErrResponseTooLarge = errors.New("dav: response exceeds limit")

type ipLookup interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// Guard validates outbound DAV targets and builds guarded HTTP clients (S-5.1).
type Guard struct {
	allow       map[string]struct{}
	connect     time.Duration
	request     time.Duration
	maxBody     int64
	maxRedirect int
	resolver    ipLookup
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
		// No proxy: with one set, the dialler would connect to the proxy and
		// the real target would never be IP-checked (architecture.md, SSRF).
		Proxy:                 nil,
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
	var permitted bool
	var lastErr error
	for _, ip := range ips {
		if !allow && blockedIP(ip.IP) {
			continue
		}
		permitted = true
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if !permitted {
		return nil, fmt.Errorf("%w: %s", ErrSSRF, host)
	}
	return nil, lastErr
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
	_, ok := g.allow[host]
	return ok
}

var (
	blockV6Whole = []netip.Prefix{
		netip.MustParsePrefix("::/96"),
		netip.MustParsePrefix("::ffff:0:0:0/96"),
		netip.MustParsePrefix("2001::/32"),
		netip.MustParsePrefix("fec0::/10"),
	}
	embedV4Prefixes = []netip.Prefix{
		netip.MustParsePrefix("64:ff9b::/96"),
		netip.MustParsePrefix("64:ff9b:1::/48"),
		netip.MustParsePrefix("2002::/16"),
	}
	prefix2002 = netip.MustParsePrefix("2002::/16")
)

func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}

	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	if addr.Is4In6() {
		addr = addr.Unmap()
	}

	for _, pfx := range blockV6Whole {
		if pfx.Contains(addr) {
			return true
		}
	}

	if v4, ok := embeddedIPv4(addr); ok {
		return blockedIPv4(v4)
	}
	if addr.Is4() {
		return blockedIPv4(addr)
	}
	if addr.Is6() {
		return privateIPv6(addr)
	}
	return false
}

func embeddedIPv4(addr netip.Addr) (netip.Addr, bool) {
	if !addr.Is6() {
		return netip.Addr{}, false
	}
	for _, pfx := range embedV4Prefixes {
		if !pfx.Contains(addr) {
			continue
		}
		b := addr.As16()
		if pfx == prefix2002 {
			return netip.AddrFrom4([4]byte{b[2], b[3], b[4], b[5]}), true
		}
		return netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}), true
	}
	if addr.Is4In6() {
		return addr.Unmap(), true
	}
	return netip.Addr{}, false
}

func blockedIPv4(addr netip.Addr) bool {
	if !addr.Is4() {
		return false
	}
	b := addr.As4()
	switch {
	case b[0] == 0: // 0.0.0.0/8
		return true
	case b[0] >= 240: // 240.0.0.0/4 and broadcast
		return true
	case b[0] == 10:
		return true
	case b[0] == 172 && b[1] >= 16 && b[1] <= 31:
		return true
	case b[0] == 192 && b[1] == 168:
		return true
	case b[0] == 127:
		return true
	case b[0] == 169 && b[1] == 254:
		return true
	case b[0] == 100 && b[1] >= 64 && b[1] <= 127: // CGNAT
		return true
	case b[0] == 192 && b[1] == 0 && b[2] == 0: // 192.0.0.0/24
		return true
	default:
		return false
	}
}

func privateIPv6(addr netip.Addr) bool {
	if !addr.Is6() {
		return false
	}
	if addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return true
	}
	b := addr.As16()
	// Unique local fc00::/7
	return b[0]&0xfe == 0xfc
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

// Read passes the body through until one byte more than the cap has been
// seen. Reading exactly the cap is a complete reply, not an overrun.
func (b *limitedBody) Read(p []byte) (int, error) {
	if b.remain < 0 {
		return 0, ErrResponseTooLarge
	}
	if int64(len(p)) > b.remain+1 {
		p = p[:b.remain+1]
	}
	n, err := b.ReadCloser.Read(p)
	b.remain -= int64(n)
	if b.remain < 0 {
		return 0, ErrResponseTooLarge
	}
	return n, err
}

// sanitizeRemoteText trims a remote error body for operator-facing messages.
func sanitizeRemoteText(b []byte) string {
	const max = 256
	if len(b) > max {
		b = b[:max]
	}
	var out strings.Builder
	for _, r := range string(b) {
		if r == '\n' || r == '\r' || r == '\t' || (r >= 32 && r < unicode.MaxASCII) {
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
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
