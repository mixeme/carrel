// Package proxy is davloom (S-27): the DAV listener that gives a device one
// account in place of every server the person connected. It answers the
// root, the principal and the home sets itself, and passes everything under
// /c/{account}/{col}/ to that collection's server through the guard, with
// hrefs rewritten on the stream. It does not import internal/web and fills
// no page of the web; its own root is view.AboutPage, as JSON.
package proxy

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// Users is what the proxy asks of the account service.
type Users interface {
	UnlockDevice(ctx context.Context, login, password string) (userID, deviceID string, dataKey [crypt.KeyLen]byte, ok bool)
	DeviceOf(userID, id string) (store.ProxyDevice, bool)
	Catalog(viewerID string, dataKey [crypt.KeyLen]byte) (sealed.Blob, bool)
	TouchDevice(id string, at time.Time)
	OnDevicesChanged(fn func(userID string))
}

// Server is the davloom listener's handler.
type Server struct {
	cfg     *conf.Config
	users   Users
	guard   *dav.Guard
	version string
	commit  string
	log     *slog.Logger
	proxies []netip.Prefix

	mu    sync.Mutex
	cache map[[32]byte]*opened // SHA-256(login NUL password) → opened key

	failLogin, failIP, device *buckets
}

// opened is a device whose wrapper was opened: kept in memory until a
// revoke, a disable, a wipe or the shutdown.
type opened struct {
	userID, deviceID, login string
	key                     [crypt.KeyLen]byte
}

// New builds the listener's handler; it wipes its cache when an account's
// devices change.
func New(cfg *conf.Config, users Users, guard *dav.Guard, version, commit string) *Server {
	s := &Server{
		cfg: cfg, users: users, guard: guard, version: version, commit: commit,
		log:       slog.Default().With("listener", "davloom"),
		cache:     map[[32]byte]*opened{},
		failLogin: newBuckets(conf.LoginRatePerAccount), failIP: newBuckets(conf.LoginRatePerIP),
		device: newBuckets(120), // a device syncing is not a person typing (plan S-27)
	}
	s.proxies = conf.ParseTrustedProxies(cfg.TrustedProxies)
	users.OnDevicesChanged(s.forget)
	return s
}

// Wipe zeroes every opened key: the shutdown (S-28).
func (s *Server) Wipe() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, o := range s.cache {
		crypt.Wipe(o.key[:])
		delete(s.cache, k)
	}
}

// forget drops the keys opened for an account.
func (s *Server) forget(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, o := range s.cache {
		if o.userID == userID {
			crypt.Wipe(o.key[:])
			delete(s.cache, k)
		}
	}
}

// hrefBase is the path the device reaches this listener under: /dav when
// the instance is published at CARREL_PUBLIC_URL (its reverse proxy strips
// /dav/ on the way in), empty on the bare port. Hrefs go out with it, or a
// client resolves them from the host root, past the proxy.
func (s *Server) hrefBase() string {
	if strings.TrimSpace(s.cfg.PublicURL) != "" {
		return "/dav"
	}
	return ""
}

// Handler is the listener's whole mux: no web routes, no cookies.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.serve)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	defer func() {
		s.log.Debug("davloom: request", "method", r.Method, "path", r.URL.EscapedPath(), "status", sw.status, "ms", time.Since(start).Milliseconds())
	}()
	// Nothing here is a page, and a client must not take it for one.
	sw.Header().Set("X-Content-Type-Options", "nosniff")
	escaped := r.URL.EscapedPath()
	if base := s.hrefBase(); base != "" {
		if rest, ok := strings.CutPrefix(escaped, base); ok && (rest == "" || strings.HasPrefix(rest, "/")) {
			escaped = rest
		}
	}
	if escaped == "" {
		escaped = "/"
	}
	// The root answers a browser without a password (S-27.4, C-13), and
	// OPTIONS says what is spoken here.
	if escaped == "/" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		s.about(sw, r)
		return
	}
	if r.Method == http.MethodOptions {
		davHeaders(sw)
		sw.WriteHeader(http.StatusOK)
		return
	}
	dev, ok := s.authenticate(sw, r)
	if !ok {
		return
	}
	s.users.TouchDevice(dev.deviceID, time.Now())
	if !s.device.allow(dev.deviceID, time.Now()) {
		http.Error(sw, "too many requests", http.StatusTooManyRequests)
		return
	}
	s.route(sw, r, dev, escaped)
}

func davHeaders(w http.ResponseWriter) {
	w.Header().Set("DAV", "1, 2, 3, calendar-access, addressbook")
	w.Header().Set("Allow", "OPTIONS, GET, HEAD, PUT, DELETE, PROPFIND, PROPPATCH, REPORT, MKCOL, MOVE, COPY, LOCK, UNLOCK")
}

// about is the root page (S-27.4): what this is, whose instance, where the
// source is — as JSON, the web's AboutPage; no collections, no names.
func (s *Server) about(w http.ResponseWriter, r *http.Request) {
	host := "this instance"
	var open view.Action
	if u, err := url.Parse(s.cfg.PublicURL); err == nil && u.Host != "" {
		host = u.Host
		open = view.Action{ID: "davloom.open", Label: "Open Carrel", Href: strings.TrimSuffix(s.cfg.PublicURL, "/") + "/", Method: "GET", Enabled: true}
	}
	page := view.AboutPage{
		Plain: view.Plain{Title: "davloom", Lede: view.Text("davloom is the DAV proxy of Carrel at " + host + ", not its web interface. " +
			"Give this address to a CalDAV, CardDAV or WebDAV client: the user name is your Carrel login, the password is the one Carrel issued for that device.")},
		Version: view.Text(s.version), Commit: view.Text(s.commit),
		Source: view.Action{ID: "about.source", Label: "Source code", Href: "https://carrel.dev", Method: "GET", Enabled: true},
		Third:  open,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	davHeaders(w)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(page)
}

// authenticate checks HTTP Basic (plan S-27): no header, a wrong password,
// a revoked device and a disabled account answer alike and take as long; a
// failure spends the sign-in buckets of this listener, not the web's.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (*opened, bool) {
	start := time.Now()
	login, password, hasAuth := r.BasicAuth()
	deny := func() (*opened, bool) {
		if d := 300*time.Millisecond - time.Since(start); d > 0 {
			time.Sleep(d)
		}
		w.Header().Set("WWW-Authenticate", `Basic realm="davloom", charset="UTF-8"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	if !hasAuth || login == "" || password == "" {
		return deny()
	}
	now := time.Now()
	ip := s.clientIP(r)
	login = strings.ToLower(strings.TrimSpace(login))
	sum := sha256.Sum256([]byte(login + "\x00" + password))
	s.mu.Lock()
	o := s.cache[sum]
	s.mu.Unlock()
	if o != nil {
		if _, live := s.users.DeviceOf(o.userID, o.deviceID); live && subtle.ConstantTimeCompare([]byte(o.login), []byte(login)) == 1 {
			return o, true
		}
		s.forget(o.userID)
	}
	if !s.failLogin.peek(login, now) || !s.failIP.peek(ip, now) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return nil, false
	}
	userID, deviceID, key, ok := s.users.UnlockDevice(r.Context(), login, password)
	if !ok {
		s.failLogin.allow(login, now)
		s.failIP.allow(ip, now)
		return deny()
	}
	o = &opened{userID: userID, deviceID: deviceID, login: login, key: key}
	s.mu.Lock()
	s.cache[sum] = o
	s.mu.Unlock()
	return o, true
}

// clientIP is the address the buckets key on: the peer, or behind a trusted
// proxy the last untrusted hop of X-Forwarded-For (as the web does).
func (s *Server) clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return r.RemoteAddr
	}
	peer = peer.Unmap()
	if !s.trusted(peer) {
		return peer.String()
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		if a = a.Unmap(); !s.trusted(a) {
			return a.String()
		}
	}
	return peer.String()
}

func (s *Server) trusted(a netip.Addr) bool {
	return conf.TrustedAddr(s.proxies, a)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// buckets are per-key fixed windows of one minute.
type buckets struct {
	mu    sync.Mutex
	limit int
	seen  map[string]window
}

type window struct {
	start time.Time
	n     int
}

func newBuckets(perMinute int) *buckets {
	return &buckets{limit: perMinute, seen: map[string]window{}}
}

// allow spends one from key's bucket; false when it is empty.
func (b *buckets) allow(key string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	w := b.seen[key]
	if now.Sub(w.start) >= time.Minute {
		w = window{start: now}
	}
	if w.n >= b.limit {
		b.seen[key] = w
		return false
	}
	w.n++
	b.seen[key] = w
	if len(b.seen) > 10000 {
		for k, v := range b.seen {
			if now.Sub(v.start) >= time.Minute {
				delete(b.seen, k)
			}
		}
	}
	return true
}

// peek reports whether key's bucket has room, spending nothing.
func (b *buckets) peek(key string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	w := b.seen[key]
	return now.Sub(w.start) >= time.Minute || w.n < b.limit
}
