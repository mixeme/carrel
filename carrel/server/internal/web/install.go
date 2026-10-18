package web

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/fanout"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// /admin/install (S-28.1): Carrel calls itself at the address declared as
// its public one and shows what a browser or a sync client will see — from
// the outside in. The client here is not the DAV guard: its only target is
// this instance's own declared URL, which the guard would rightly refuse as
// a loop. Nothing is changed but the address, saved in settings.env; the run
// is journalled. Eleven rows, a closed list; a row that cannot be measured
// from outside is CheckUnknown with what was seen and the advice.

const probeHeader = "X-Carrel-Probe"

// probeSeen is what a self-request looked like when it arrived — after the
// reverse proxy, before any handler.
type probeSeen struct {
	Host, ForwardedHost, ForwardedProto, ForwardedFor string
	Peer, ClientIP                                    string
	Trusted, Secure, Cookie                           bool
}

// probes are the pending self-requests: a random token each, answered once.
type probes struct {
	mu   sync.Mutex
	wait map[string]chan probeSeen
}

func (p *probes) expect() (string, chan probeSeen) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	ch := make(chan probeSeen, 4)
	p.mu.Lock()
	if p.wait == nil {
		p.wait = map[string]chan probeSeen{}
	}
	p.wait[tok] = ch
	p.mu.Unlock()
	return tok, ch
}

func (p *probes) done(tok string) {
	p.mu.Lock()
	delete(p.wait, tok)
	p.mu.Unlock()
}

// observe notes a self-request of a running check; any other request, and a
// header with a token nobody waits for, pass untouched.
func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tok := r.Header.Get(probeHeader); tok != "" {
			s.probes.mu.Lock()
			ch := s.probes.wait[tok]
			s.probes.mu.Unlock()
			if ch != nil {
				peer, _ := remoteAddr(r)
				_, cookieErr := r.Cookie(cookieName)
				seen := probeSeen{
					Host: r.Host, ForwardedHost: r.Header.Get("X-Forwarded-Host"), ForwardedProto: r.Header.Get("X-Forwarded-Proto"),
					ForwardedFor: r.Header.Get("X-Forwarded-For"), Peer: peer.String(), ClientIP: s.clientIP(r),
					Trusted: s.fromTrustedProxy(r), Secure: s.secureCookies(r), Cookie: cookieErr == nil,
				}
				select {
				case ch <- seen:
				default:
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) getAdminInstall(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	s.writePage(w, r, http.StatusOK, s.installPage(sess, strings.TrimSpace(s.cfg.PublicURL), nil, ""))
}

// postAdminInstall takes the address (url=), keeps it and runs the check.
func (s *Server) postAdminInstall(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	raw := strings.TrimSpace(r.FormValue("url"))
	if raw == "" {
		raw = strings.TrimSpace(s.cfg.PublicURL)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		// Refused (s-admin-26): the field in error, why under the head, no
		// table of a run before — it was of another address.
		const why = "Give the address people open Carrel at, like https://carrel.example.org."
		page := s.installPage(sess, raw, nil, why)
		page.Form.Errors = []view.FieldError{{Name: "url", Message: why}}
		for i := range page.Form.Sets[0].Rows {
			if page.Form.Sets[0].Rows[i].Name == "url" {
				page.Form.Sets[0].Rows[i].Error = why
			}
		}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	origin := strings.TrimSuffix(u.Scheme+"://"+u.Host+u.EscapedPath(), "/")
	note := ""
	if origin != strings.TrimSuffix(s.cfg.PublicURL, "/") {
		if err := conf.SaveSetting(s.cfg.DataDir, "CARREL_PUBLIC_URL", origin); err != nil {
			note = "The address could not be written to settings.env: " + err.Error()
		} else if conf.FromEnvironment("CARREL_PUBLIC_URL") {
			note = "The environment of the process sets CARREL_PUBLIC_URL too, and it wins at the next start; this run uses the address given here."
		}
		s.cfg.PublicURL = origin
	}
	cookie, _ := r.Cookie(cookieName)
	start := time.Now()
	rows := s.runInstallCheck(r.Context(), sess, origin, cookie)
	took := time.Since(start)
	s.markChecked(rows)
	bad := 0
	for _, row := range rows {
		if row.State == view.CheckBad {
			bad++
		}
	}
	s.users.Journal(sess.AccountID(), "install.check", fmt.Sprintf("%s: %d of %d broken", origin, bad, len(rows)))
	page := s.installPage(sess, origin, rows, note)
	page.Took = text(tookText(took)) // how long it ran, «4.2 s» (Ад30; s-admin-4)
	s.writePage(w, r, http.StatusOK, page)
}

// installPage is the page of the check: in its head the address it asks
// and Run the check, one form (Ад29; s-admin-4).
func (s *Server) installPage(sess *session.Session, publicURL string, rows []view.InstallRow, note string) view.AdminInstallPage {
	form := withPrimary(s.plainForm("/admin/install", "POST", csrfField(sess),
		view.FormRow{Name: "url", Label: "Base address", Kind: view.InputURL, Value: text(publicURL), Mono: true}),
		"admin.install.run", "Run the check")
	page := view.AdminInstallPage{Shell: s.withRail(s.buildShell(sess, "Installation check"), s.adminRail(sess, "/admin/install")), Form: form, Rows: rows}
	// What went wrong with the address, a message under the head
	// (s-admin-26; settings.env and the environment, Typecase's note 79).
	if note != "" {
		page.Notices = append(page.Notices, view.Notice{Role: view.NoticeAlert, Body: text(note)})
	}
	// Without a table, what the check does (s-admin-25, -26).
	if len(rows) == 0 {
		page.Note = "The check runs from outside inwards: Carrel calls itself at the base address, so it sees exactly what a browser or DAVx5 will. " +
			"Nothing is changed and nothing is stored, apart from the audit line saying the check was run."
	}
	return page
}

// runInstallCheck is the eleven rows, in order.
func (s *Server) runInstallCheck(ctx context.Context, sess *session.Session, origin string, cookie *http.Cookie) []view.InstallRow {
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var rows []view.InstallRow
	row := func(check string, st view.CheckState, value, advice string) {
		rows = append(rows, view.InstallRow{Check: text(check), State: st, Value: text(value), Advice: text(advice)})
	}

	// 1. Reachable, and what the request looked like when it came in.
	tok, seenCh := s.probes.expect()
	defer s.probes.done(tok)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/healthz", nil)
	req.Header.Set(probeHeader, tok)
	start := time.Now()
	resp, err := client.Do(req)
	took := time.Since(start)
	var health *http.Response
	switch {
	case err != nil:
		row("Reachable from outside", view.CheckBad, "GET "+origin+"/healthz → "+err.Error(),
			"The address does not reach this Carrel. Check DNS, the proxy's server block and that it forwards to this port.")
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
		resp.Body.Close()
		health = resp
		if resp.StatusCode == 200 && strings.TrimSpace(string(body)) == "ok" {
			row("Reachable from outside", view.CheckOK, fmt.Sprintf("GET %s/healthz → 200 · %d ms", origin, took.Milliseconds()), "")
		} else {
			row("Reachable from outside", view.CheckBad, fmt.Sprintf("GET %s/healthz → %d", origin, resp.StatusCode),
				"Something else answers at this address: the proxy forwards it elsewhere, or adds a path.")
		}
	}
	var seen *probeSeen
	select {
	case v := <-seenCh:
		seen = &v
	case <-time.After(200 * time.Millisecond):
	}

	// 2. Certificate.
	switch {
	case !strings.HasPrefix(origin, "https://"):
		row("Certificate", view.CheckBad, "plain http: passwords and data cross the network in clear", "Serve Carrel over https at the proxy.")
	case health == nil || health.TLS == nil || len(health.TLS.PeerCertificates) == 0:
		row("Certificate", view.CheckBad, "no certificate could be read", "The TLS handshake failed; see the first row.")
	default:
		leaf := health.TLS.PeerCertificates[0]
		row("Certificate", view.CheckOK, fmt.Sprintf("%s · until %s · chain complete · name matches", issuerName(health.TLS), leaf.NotAfter.Format("2006-01-02")), "")
	}

	// 3–5. What the proxy passed on.
	if seen == nil {
		for _, c := range []string{"Base address", "Scheme behind the proxy", "Client address"} {
			row(c, view.CheckUnknown, "the request did not reach Carrel", "Fix the first row first.")
		}
	} else {
		u, _ := url.Parse(origin)
		host := seen.Host
		if seen.ForwardedHost != "" {
			host = seen.ForwardedHost
		}
		if strings.EqualFold(host, u.Host) {
			row("Base address", view.CheckOK, "Host "+seen.Host+" · X-Forwarded-Host "+dash(seen.ForwardedHost)+" match the address", "")
		} else {
			row("Base address", view.CheckBad, "Host "+seen.Host+" · X-Forwarded-Host "+dash(seen.ForwardedHost)+" · the address says "+u.Host,
				"Pass the original host on: proxy_set_header Host $host (or X-Forwarded-Host).")
		}
		wantHTTPS := u.Scheme == "https"
		switch {
		case wantHTTPS && !seen.Secure:
			row("Scheme behind the proxy", view.CheckBad, "X-Forwarded-Proto: "+dash(seen.ForwardedProto)+" · links and cookies are built as http",
				"Set X-Forwarded-Proto $scheme at the proxy, always rather than when empty, and list the proxy in CARREL_TRUSTED_PROXIES.")
		default:
			row("Scheme behind the proxy", view.CheckOK, "X-Forwarded-Proto: "+dash(seen.ForwardedProto)+" · links are built as "+u.Scheme, "")
		}
		switch {
		case seen.ForwardedFor == "":
			row("Client address", view.CheckBad, "no X-Forwarded-For · limits count "+seen.ClientIP+", every visitor alike",
				"Set X-Forwarded-For at the proxy.")
		case !seen.Trusted:
			row("Client address", view.CheckBad, "X-Forwarded-For: "+seen.ForwardedFor+" · the proxy "+seen.Peer+" is not trusted, limits count it",
				"Add the proxy's address to CARREL_TRUSTED_PROXIES.")
		default:
			row("Client address", view.CheckOK, "X-Forwarded-For: "+seen.ForwardedFor+" · limits count "+seen.ClientIP+", not the proxy", "")
		}
	}

	// 6. The event stream: headers and the first byte at once (S-15).
	rows = append(rows, s.checkSSE(ctx, client, sess, origin, cookie))

	// 7. Upload size: a 21 MiB body to /healthz, which answers 405 after it.
	body := io.LimitReader(zeros{}, 21<<20)
	upReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, origin+"/healthz", body)
	upReq.ContentLength = 21 << 20
	upResp, err := (&http.Client{Timeout: 60 * time.Second}).Do(upReq)
	switch {
	case err != nil:
		row("Upload size", view.CheckBad, "21 MB probe → the connection was cut: "+err.Error(), "Raise or lift client_max_body_size at the proxy: Carrel has no ceiling of its own.")
	case upResp.StatusCode == http.StatusRequestEntityTooLarge:
		upResp.Body.Close()
		row("Upload size", view.CheckBad, "21 MB probe → 413 from the proxy; Carrel streams and has no size ceiling of its own",
			"client_max_body_size — the proxy is cutting the body.")
	default:
		upResp.Body.Close()
		row("Upload size", view.CheckOK, fmt.Sprintf("21 MB probe → %d from Carrel · the proxy let the body through", upResp.StatusCode), "")
	}

	// 8. Long requests: the proxy's ceiling cannot be measured from outside.
	row("Long request timeout", view.CheckUnknown, fmt.Sprintf("a fanout may take up to %s; the proxy's ceiling cannot be measured from outside", s.cfg.ProgressTotalTimeout),
		"proxy_read_timeout ≥ 60s")

	// 9. Cookie.
	switch {
	case seen == nil:
		row("Cookie", view.CheckUnknown, "the request did not reach Carrel", "Fix the first row first.")
	case strings.HasPrefix(origin, "https://") && !seen.Secure:
		row("Cookie", view.CheckBad, "HttpOnly, SameSite=Lax · without Secure", "The scheme row: Carrel does not see https, so the cookie is not marked Secure.")
	default:
		flags := "HttpOnly, SameSite=Lax"
		if seen.Secure {
			flags = "Secure, " + flags
		}
		row("Cookie", view.CheckOK, flags+" · came back", "")
	}

	// 10. Clock.
	tz := s.cfg.Timezone
	if tz == "" {
		tz = "UTC"
	}
	if health != nil {
		if d, err := http.ParseTime(health.Header.Get("Date")); err == nil {
			skew := time.Since(d).Round(100 * time.Millisecond)
			if skew < 0 {
				skew = -skew
			}
			if skew <= 5*time.Second {
				row("Clock", view.CheckOK, fmt.Sprintf("%s between the proxy's Date and this clock · TZ %s", skew, tz), "")
			} else {
				row("Clock", view.CheckBad, fmt.Sprintf("%s between the proxy's Date and this clock · TZ %s", skew, tz), "Keep the host's clock in sync (NTP).")
			}
		} else {
			row("Clock", view.CheckUnknown, "no Date header came back · TZ "+tz, "")
		}
	} else {
		row("Clock", view.CheckUnknown, "the request did not reach Carrel · TZ "+tz, "")
	}

	// 11. Data folder, from the inside.
	probe := filepath.Join(s.cfg.DataDir, ".install-check")
	if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
		row("Data folder", view.CheckBad, s.cfg.DataDir+" · not writable: "+err.Error(), "Give the process write access to CARREL_DATA_DIR.")
	} else {
		_ = os.Remove(probe)
		if free, ok := diskFree(s.cfg.DataDir); ok {
			st, advice := view.CheckOK, ""
			if free < 1<<30 {
				st, advice = view.CheckBad, "Less than 1 GB left: the state and temporary files need room."
			}
			row("Data folder", st, fmt.Sprintf("%s · writable · %s free", s.cfg.DataDir, formatSize(int64(free))), advice)
		} else {
			row("Data folder", view.CheckOK, s.cfg.DataDir+" · writable", "")
		}
	}
	return rows
}

// checkSSE opens the progress stream of a finished task of this session with
// the administrator's own cookie and times the first byte.
func (s *Server) checkSSE(ctx context.Context, client *http.Client, sess *session.Session, origin string, cookie *http.Cookie) view.InstallRow {
	name := text("Event stream (SSE)")
	if s.cfg.ProgressMode == "poll" {
		return view.InstallRow{Check: name, State: view.CheckUnknown, Value: "CARREL_PROGRESS_MODE=poll: no stream is used", Advice: ""}
	}
	id, err := user.NewTaskID()
	if err != nil {
		return view.InstallRow{Check: name, State: view.CheckUnknown, Value: text(err.Error())}
	}
	opts := s.searchOptions(sess)
	opts.Finished = nil // not a search of the history
	task, err := fanout.Start(id, fanout.Query{Text: "installation check"}, nil, opts)
	if err != nil {
		return view.InstallRow{Check: name, State: view.CheckUnknown, Value: text(err.Error())}
	}
	sess.KeepTask(task)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/app/search/progress/"+id, nil)
	if cookie != nil {
		req.AddCookie(&http.Cookie{Name: cookie.Name, Value: cookie.Value})
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return view.InstallRow{Check: name, State: view.CheckBad, Value: text("the stream could not be opened: " + err.Error())}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return view.InstallRow{Check: name, State: view.CheckBad, Value: text(fmt.Sprintf("GET the stream → %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))),
			Advice: "The session cookie did not come through, or the proxy answers for this path itself."}
	}
	one := make([]byte, 1)
	_, _ = resp.Body.Read(one)
	first := time.Since(start)
	if first > 2*time.Second {
		return view.InstallRow{Check: name, State: view.CheckBad, Value: text(fmt.Sprintf("first byte after %.1f s — the response is buffered", first.Seconds())),
			Advice: "nginx: proxy_buffering off for /app/search/progress/"}
	}
	return view.InstallRow{Check: name, State: view.CheckOK, Value: text(fmt.Sprintf("first byte after %d ms", first.Milliseconds()))}
}

func issuerName(st *tls.ConnectionState) string {
	c := st.PeerCertificates[0]
	if c.Issuer.CommonName != "" {
		return c.Issuer.CommonName
	}
	if len(c.Issuer.Organization) > 0 {
		return c.Issuer.Organization[0]
	}
	return "issuer unnamed"
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// zeros is an endless body of zero bytes.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
