package proxy

import (
	"bytes"
	"html"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
)

// Below /c/{account}/{col}/ everything goes to that collection's server
// through the guard (S-27.3). The bytes of records and files pass as they
// are (S-7); only XML has its hrefs rewritten, on the stream, both ways.

// maxRequestXML is the largest XML body a device may send: a multiget of
// thousands of hrefs is well under it.
const maxRequestXML = 10 << 20

// forwardedMethods are the methods davloom passes on — the Allow of
// davHeaders — and no other: a method a client invents is answered here,
// never sent to the collection's server with the person's credentials
// (S-27.3, audit №3, Ф3-8).
var forwardedMethods = map[string]bool{
	http.MethodOptions: true, http.MethodGet: true, http.MethodHead: true, http.MethodPut: true, http.MethodDelete: true,
	"PROPFIND": true, "PROPPATCH": true, "REPORT": true, "MKCOL": true, "MOVE": true, "COPY": true, "LOCK": true, "UNLOCK": true,
}

// writeMethods change something on the collection's server. A device that is
// read only is refused them here, before anything is sent (owner's decision
// 24.09.2026). LOCK is among them: a lock is taken to write, and on some
// servers it makes an empty resource.
var writeMethods = map[string]bool{
	http.MethodPut: true, http.MethodDelete: true, "MOVE": true, "COPY": true,
	"PROPPATCH": true, "MKCOL": true, "MKCALENDAR": true, "LOCK": true,
}

// requestHeaders are the headers a device's request carries on: what DAV
// needs and nothing of the device's credentials.
var requestHeaders = []string{"Content-Type", "Depth", "If-Match", "If-None-Match", "If-Modified-Since", "Overwrite",
	"Prefer", "Brief", "Range", "Accept", "Timeout", "Lock-Token"}

// responseHeadersDropped never reach the device.
var responseHeadersDropped = map[string]bool{
	"Connection": true, "Keep-Alive": true, "Proxy-Authenticate": true, "Transfer-Encoding": true, "Upgrade": true,
	"Set-Cookie": true, "Www-Authenticate": true, "Content-Length": true, "Location": true, "Content-Location": true,
	// davloom's own sandbox stands (А4-2): the far server's policy and
	// framing do not travel.
	"Content-Security-Policy": true, "Content-Security-Policy-Report-Only": true, "X-Frame-Options": true,
}

func isXML(contentType string) bool {
	mt, _, _ := mime.ParseMediaType(contentType)
	return mt == "application/xml" || mt == "text/xml" || mt == "application/dav+xml"
}

// target is one address under /c/: the collection and the path under it.
type target struct {
	sc   scoped
	rest string // escaped segments under the collection; "" for itself
}

// parseTarget reads account, collection and path; a collection outside the
// device's scope is not found, like one that does not exist (plan S-27).
func parseTarget(blob sealed.Blob, d store.ProxyDevice, rest string) (target, bool) {
	account, after, _ := strings.Cut(rest, "/")
	colEsc, tail, _ := strings.Cut(after, "/")
	colPath, err := url.PathUnescape(colEsc)
	if err != nil || account == "" || colPath == "" {
		return target{}, false
	}
	var segs []string
	for _, seg := range strings.Split(tail, "/") {
		if seg == "" {
			continue
		}
		raw, err := url.PathUnescape(seg)
		if err != nil || raw == "." || raw == ".." || strings.ContainsAny(raw, "/\\") {
			if err == nil && raw != "." {
				slog.Warn("davloom: a path that leaves its collection was refused", "listener", "davloom", "path", rest)
			}
			return target{}, false
		}
		segs = append(segs, url.PathEscape(raw))
	}
	for _, sc := range scope(blob, d) {
		if sc.conn.ID == account && sc.col.Path == colPath {
			t := target{sc: sc, rest: strings.Join(segs, "/")}
			if strings.HasSuffix(tail, "/") && t.rest != "" {
				t.rest += "/"
			}
			return t, true
		}
	}
	return target{}, false
}

// upstream is the path on the collection's server.
func (t target) upstream() string {
	return strings.TrimSuffix(t.sc.col.Path, "/") + "/" + t.rest
}

func (s *Server) forward(w http.ResponseWriter, r *http.Request, blob sealed.Blob, d store.ProxyDevice, rest string) {
	if !forwardedMethods[r.Method] {
		davHeaders(w)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	t, ok := parseTarget(blob, d, rest)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if d.ReadOnly && writeMethods[r.Method] {
		// RFC 3744 §7.1.1: what is missing is a privilege, and the answer
		// says which.
		w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
		davHeaders(w)
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>`+
			`<d:error xmlns:d="DAV:"><d:need-privileges><d:resource><d:href>`+html.EscapeString(r.URL.Path)+
			`</d:href><d:privilege><d:write/></d:privilege></d:resource></d:need-privileges></d:error>`)
		return
	}
	conn := t.sc.conn
	client, err := dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if err != nil {
		http.Error(w, "the server of this collection cannot be reached", http.StatusBadGateway)
		return
	}
	local := s.colHref(conn.ID, t.sc.col.Path)
	far := strings.TrimSuffix(t.sc.col.Path, "/") + "/"
	farURL := client.URL(far)
	toFar := map[string]string{local: far, strings.TrimPrefix(local, s.hrefBase()): far}
	toLocal := map[string]string{farURL: local, far: local}
	if esc := escapePath(far); esc != far {
		toLocal[esc] = local
		toLocal[client.URL(esc)] = local
	}

	header := http.Header{}
	for _, h := range requestHeaders {
		for _, v := range r.Header.Values(h) {
			header.Add(h, v)
		}
	}
	if ifh := r.Header.Get("If"); ifh != "" {
		header.Set("If", replaceAll(ifh, toFar))
	}
	if dest := r.Header.Get("Destination"); dest != "" {
		u, err := url.Parse(dest)
		if err != nil {
			http.Error(w, "bad destination", http.StatusBadRequest)
			return
		}
		p := u.EscapedPath()
		if base := s.hrefBase(); base != "" {
			p = strings.TrimPrefix(p, base)
		}
		drest, ok := strings.CutPrefix(p, "/c/")
		dt, inScope := parseTarget(blob, d, drest)
		switch {
		case !ok || !inScope:
			http.Error(w, "the destination is not a collection of this device", http.StatusForbidden)
			return
		case dt.sc.conn.ID != conn.ID:
			// Two servers: nothing moves between them in one request.
			http.Error(w, "the destination is on another server", http.StatusBadGateway)
			return
		}
		header.Set("Destination", client.URL(dt.upstream()))
	}
	var body io.Reader
	if r.Body != nil && r.ContentLength != 0 {
		body = r.Body
	}
	length := r.ContentLength
	if body != nil && isXML(r.Header.Get("Content-Type")) {
		// A request body is small — a query, a multiget — and goes out with
		// its length: a chunked REPORT is refused by some servers (SV-5).
		// The response, which can be big, stays a stream.
		data, err := io.ReadAll(io.LimitReader(newRewriter(body, toFar), maxRequestXML+1))
		if err != nil || int64(len(data)) > maxRequestXML {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		body, length = bytes.NewReader(data), int64(len(data))
	}
	file := r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodPut
	resp, err := client.Forward(r.Context(), r.Method, t.upstream(), header, body, length, file)
	if err != nil {
		s.log.Warn("davloom: upstream failed", "account", conn.ID, "method", r.Method, "error", err)
		http.Error(w, "the server of this collection did not answer", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		// The device's own password was right; it is the connection's saved
		// one the far server refused. A 401 would make the device ask again.
		http.Error(w, "the server of this collection refused the saved credentials", http.StatusBadGateway)
		return
	}
	for k, vs := range resp.Header {
		if responseHeadersDropped[http.CanonicalHeaderKey(k)] {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	for _, h := range []string{"Location", "Content-Location"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, replaceAll(v, toLocal))
		}
	}
	var out io.Reader = resp.Body
	if isXML(resp.Header.Get("Content-Type")) {
		out = newRewriter(resp.Body, toLocal)
	} else if resp.ContentLength >= 0 {
		w.Header().Set("Content-Length", resp.Header.Get("Content-Length"))
	}
	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}
	buf := make([]byte, 32<<10)
	flusher, _ := w.(http.Flusher)
	for {
		n, err := out.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}

// escapePath is a path as a server would write it in an href: each
// segment escaped.
func escapePath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// replaceAll replaces in one pass, the longest pattern first at each place:
// "/dav/c/…" before the "/c/…" inside it.
func replaceAll(s string, pairs map[string]string) string {
	froms := make([]string, 0, len(pairs))
	for from := range pairs {
		if from != "" {
			froms = append(froms, from)
		}
	}
	sort.Slice(froms, func(i, j int) bool { return len(froms[i]) > len(froms[j]) })
	args := make([]string, 0, 2*len(froms))
	for _, from := range froms {
		args = append(args, from, pairs[from])
	}
	return strings.NewReplacer(args...).Replace(s)
}
