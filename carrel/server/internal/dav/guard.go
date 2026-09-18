package dav

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"syscall"
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
	return g.httpClient("")
}

// HTTPClientWithCertPin returns a client that accepts a leaf certificate when
// standard chain verification fails but the SHA-256 fingerprint matches pin
// (F-13). Empty pin keeps standard verification only.
func (g *Guard) HTTPClientWithCertPin(pinnedFingerprint string) *http.Client {
	return g.httpClient(pinnedFingerprint)
}

func (g *Guard) httpClient(pinnedFingerprint string) *http.Client {
	transport := &http.Transport{
		// No proxy: with one set, the dialler would connect to the proxy and
		// the real target would never be IP-checked (architecture.md, SSRF).
		Proxy:                 nil,
		DialContext:           g.dialContext,
		DialTLSContext:        g.dialTLSContext(pinnedFingerprint),
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	// No http.Client.Timeout: it counts the body too and cut every file that
	// took longer than a request (S-17). The transport keeps the deadline
	// instead, and for a stream counts silence rather than the whole.
	return &http.Client{
		Transport: &limitTransport{base: transport, max: g.maxBody, request: g.request},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= g.maxRedirect {
				return fmt.Errorf("dav: too many redirects")
			}
			if httpsToHTTP(via[0].URL, req.URL) {
				return fmt.Errorf("dav: redirect downgrades HTTPS to HTTP")
			}
			return nil
		},
	}
}

func httpsToHTTP(from, to *url.URL) bool {
	if from == nil || to == nil {
		return false
	}
	return strings.EqualFold(from.Scheme, "https") && strings.EqualFold(to.Scheme, "http")
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

func (g *Guard) dialTLSContext(pinnedFingerprint string) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		conn, err := g.dialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		cfg := tlsConfigWithPin(host, pinnedFingerprint)
		return tls.Client(conn, cfg), nil
	}
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
	base    http.RoundTripper
	max     int64
	request time.Duration // CARREL_DAV_REQUEST_TIMEOUT
}

// RoundTrip sends the request and, when a connection the pool had kept turns
// out to be closed, sends it once more (docs/servers.md, SV-14, S-5.1). Go
// replays only the methods it knows as safe, and PROPFIND and REPORT are not
// among them; the pool lives in this guard, so every DAV client of the process
// shares it and inherits the race.
//
// A read goes again whatever the pool said. A write goes again only when the
// connection came out of the pool idle: the server had closed its side before
// the request, so it never saw it. A connection dialled for this request is
// not retried — there the error says nothing about what the server did with
// it. The second reason it is safe: every write Carrel sends is conditional
// (S-8) or idempotent, so one that did land answers the retry with a failed
// precondition instead of doubling.
func (t *limitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err, fromIdle := t.attempt(req)
	// A connection that ended before the answer says why it was not sent
	// again. Such a failure is rare, does not repeat, and is read months later
	// out of a log: "EOF" alone leaves the reader guessing which rule applied.
	switch {
	case err == nil || !idleClosed(err):
		return resp, err
	case !rewindable(req):
		return resp, fmt.Errorf("%w (not sent again: its body cannot be produced a second time)", err)
	case !readOnly(req) && !fromIdle:
		return resp, fmt.Errorf("%w (not sent again: a write on a connection dialled for it, "+
			"so what the server did with it is unknown)", err)
	}
	again, rerr := rewound(req)
	if rerr != nil {
		return resp, fmt.Errorf("%w (not sent again: the body would not rewind: %v)", err, rerr)
	}
	resp, second, _ := t.attempt(again)
	if second != nil && idleClosed(second) {
		return resp, fmt.Errorf("%w (sent again after a connection closed before the answer, and it ended the same way)", second)
	}
	return resp, second
}

// readOnly reports whether the method only reads, so sending it again changes
// nothing on the server whatever happened to the first one.
func readOnly(req *http.Request) bool {
	switch req.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, "PROPFIND", "REPORT":
		return true
	default:
		return false
	}
}

// rewindable reports whether the request can produce its body a second time.
// A body streamed from somewhere else — a file on its way to another server —
// cannot, and is never sent twice.
func rewindable(req *http.Request) bool {
	return req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
}

// idleClosed reports the errors a connection closed before the answer gives.
// A timeout, a refusal and a cancelled request are not among them.
func idleClosed(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, net.ErrClosed)
}

func rewound(req *http.Request) (*http.Request, error) {
	out := req.Clone(req.Context())
	if req.GetBody == nil {
		out.Body = nil
		return out, nil
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	out.Body = body
	return out, nil
}

// attempt sends the request once. fromIdle is true when the connection it
// went out on came from the pool having sat idle — the one case where a
// connection closed mid-flight says the server never saw the request.
func (t *limitTransport) attempt(req *http.Request) (resp *http.Response, err error, fromIdle bool) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) { fromIdle = info.Reused && info.WasIdle },
	}))
	var w *watchdog
	if t.request > 0 {
		var ctx context.Context
		ctx, w = watch(req.Context(), t.request, streaming(req))
		req = req.WithContext(ctx)
		if w.stream && req.Body != nil && req.Body != http.NoBody {
			req.Body = &watchedBody{ReadCloser: req.Body, w: w, out: true}
		}
	}
	resp, err = base.RoundTrip(req)
	if err != nil {
		if w != nil {
			w.stop()
			err = w.err(err)
		}
		return nil, err, fromIdle
	}
	if w != nil {
		if resp.Body == nil {
			w.stop()
		} else {
			resp.Body = &watchedBody{ReadCloser: resp.Body, w: w}
		}
	}
	if resp.Body != nil && t.max > 0 && !uncapped(req) {
		resp.Body = &limitedBody{ReadCloser: resp.Body, remain: t.max}
	}
	return resp, nil, fromIdle
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

// ErrTimeout is what a request cut by CARREL_DAV_REQUEST_TIMEOUT fails with.
// It is a timeout to net.Error and context.DeadlineExceeded to errors.Is, as
// http.Client.Timeout was.
var ErrTimeout error = timeoutError{}

type timeoutError struct{}

func (timeoutError) Error() string   { return "dav: request timed out" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
func (timeoutError) Is(target error) bool {
	return target == context.DeadlineExceeded
}

// watchdog cancels a request that took too long. A plain request has one
// deadline for all of it, body included. A stream (Stream) is cut only after
// that long without a byte: each byte that goes out or comes in starts the
// count again, so a large file on a slow line passes and a silent server does
// not hold the connection.
type watchdog struct {
	mu     sync.Mutex
	timer  *time.Timer
	d      time.Duration
	stream bool
	fired  bool
	done   bool
	cancel context.CancelFunc
}

func watch(parent context.Context, d time.Duration, stream bool) (context.Context, *watchdog) {
	ctx, cancel := context.WithCancel(parent)
	w := &watchdog{d: d, stream: stream, cancel: cancel}
	w.mu.Lock()
	w.timer = time.AfterFunc(d, w.fire)
	w.mu.Unlock()
	return ctx, w
}

func (w *watchdog) fire() {
	w.mu.Lock()
	if !w.done {
		w.fired = true
	}
	w.mu.Unlock()
	w.cancel()
}

// progress starts the count again for a stream; a plain request keeps its
// deadline.
func (w *watchdog) progress() {
	if !w.stream {
		return
	}
	w.mu.Lock()
	if !w.fired && !w.done {
		w.timer.Reset(w.d)
	}
	w.mu.Unlock()
}

// stop ends the request: the timer and its context go.
func (w *watchdog) stop() {
	w.mu.Lock()
	w.done = true
	w.timer.Stop()
	w.mu.Unlock()
	w.cancel()
}

// err names a failure the watchdog caused; any other passes as it is.
func (w *watchdog) err(err error) error {
	w.mu.Lock()
	fired := w.fired
	w.mu.Unlock()
	if fired {
		return ErrTimeout
	}
	return err
}

// watchedBody reports bytes to the watchdog: a request body as the transport
// sends it (out), a response body as the caller reads it, which also ends the
// request at EOF or Close.
type watchedBody struct {
	io.ReadCloser
	w   *watchdog
	out bool
}

func (b *watchedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.w.progress()
	}
	switch {
	case err == io.EOF:
		if !b.out {
			b.w.stop()
		}
	case err != nil:
		err = b.w.err(err)
	}
	return n, err
}

func (b *watchedBody) Close() error {
	err := b.ReadCloser.Close()
	if !b.out {
		b.w.stop()
	}
	return err
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
	// RFC 6764: well-known URIs are anchored at the origin, not the entered path.
	u.Path = wellKnown
	u.RawQuery = ""
	u.Fragment = ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		// The trace says why the step failed: a redirect refused by the guard
		// (an https well-known sending to http, docs/servers.md, SV-10) reads as
		// that refusal, not as an empty detail.
		reason := err.Error()
		var ue *url.Error
		if errors.As(err, &ue) {
			reason = ue.Err.Error()
		}
		return "", fmt.Sprintf("GET %s: %s", wellKnown, reason), err
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
