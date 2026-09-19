package proxy

import (
	"context"
	"encoding/xml"
	"html"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// The three levels answered here (S-27.3): the root, the principal and the
// home sets. A collection is /c/{account}/{col}/ — the connection's ID and
// the catalog path in one segment, as in the web — so its address stays the
// same across reconnection, restarts and the order of connections.

// home is one home set: the kinds whose collections it lists.
type home struct {
	path  string
	kinds []view.Kind
	name  string
}

var homes = []home{
	{"/cal/", []view.Kind{view.KindEvent, view.KindTask, view.KindNote}, "Calendars"},
	{"/card/", []view.Kind{view.KindContact}, "Address books"},
	{"/files/", []view.Kind{view.KindFile}, "Files"},
}

// scoped is one collection of the device's scope, live in the catalog.
type scoped struct {
	conn  sealed.Connection
	col   sealed.Collection
	kinds []view.Kind // the collection's kinds the device may see
}

// scope is the device's collections: its ScopeCols that the catalog holds
// live, of a connection of the person's own, with the kinds the device and
// the collection share.
func scope(blob sealed.Blob, d store.ProxyDevice) []scoped {
	allowed := map[view.Kind]bool{}
	for _, k := range d.ScopeKinds {
		allowed[view.Kind(k)] = true
	}
	var out []scoped
	for _, c := range d.ScopeCols {
		conn, okConn := blob.Connection(c.AccountID)
		col, okCol := blob.Collection(c.AccountID, c.Path)
		if !okConn || !okCol || conn.Feed || col.Gone {
			continue
		}
		var kinds []view.Kind
		for _, k := range col.Kinds {
			if allowed[k] {
				kinds = append(kinds, k)
			}
		}
		if len(kinds) > 0 {
			out = append(out, scoped{conn: conn, col: col, kinds: kinds})
		}
	}
	return out
}

func (sc scoped) has(kinds []view.Kind) bool {
	for _, k := range sc.kinds {
		for _, want := range kinds {
			if k == want {
				return true
			}
		}
	}
	return false
}

// colHref is the collection's address on this listener.
func (s *Server) colHref(account, colPath string) string {
	return s.hrefBase() + "/c/" + account + "/" + url.PathEscape(colPath) + "/"
}

func (s *Server) route(w http.ResponseWriter, r *http.Request, o *opened, escaped string) {
	d, live := s.users.DeviceOf(o.userID, o.deviceID)
	blob, ok := s.users.Catalog(o.userID, o.key)
	if !live || !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if rest, ok := strings.CutPrefix(escaped, "/c/"); ok {
		s.forward(w, r, blob, d, rest)
		return
	}
	var synth string
	switch escaped {
	case "/", "/principal/", "/cal/", "/card/", "/files/":
		synth = escaped
	case "/principal", "/cal", "/card", "/files":
		synth = escaped + "/"
	default:
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if r.Method != "PROPFIND" {
		davHeaders(w)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	depth1 := r.Header.Get("Depth") != "0"
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n" +
		`<d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:card="urn:ietf:params:xml:ns:carddav" xmlns:cs="http://calendarserver.org/ns/" xmlns:ic="http://apple.com/ns/ical/">`)
	base := s.hrefBase()
	principal := base + "/principal/"
	self := func(href, name, types string, extra string) {
		b.WriteString("<d:response><d:href>" + html.EscapeString(href) + "</d:href><d:propstat><d:prop>" +
			"<d:resourcetype>" + types + "</d:resourcetype><d:displayname>" + html.EscapeString(name) + "</d:displayname>" +
			"<d:current-user-principal><d:href>" + html.EscapeString(principal) + "</d:href></d:current-user-principal>" + extra +
			"</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>")
	}
	homeSets := "<c:calendar-home-set><d:href>" + base + "/cal/</d:href></c:calendar-home-set>" +
		"<card:addressbook-home-set><d:href>" + base + "/card/</d:href></card:addressbook-home-set>"
	switch synth {
	case "/":
		self(base+"/", "davloom", "<d:collection/>", "")
		if depth1 {
			self(principal, o.login, "<d:collection/><d:principal/>", homeSets)
		}
	case "/principal/":
		self(principal, o.login, "<d:collection/><d:principal/>", homeSets+
			"<d:principal-URL><d:href>"+principal+"</d:href></d:principal-URL>")
	default:
		var h home
		for _, x := range homes {
			if x.path == synth {
				h = x
			}
		}
		self(base+h.path, h.name, "<d:collection/>", "")
		if depth1 {
			var list []scoped
			for _, sc := range scope(blob, d) {
				if sc.has(h.kinds) {
					list = append(list, sc)
				}
			}
			tags := s.ctags(r.Context(), list)
			for i, sc := range list {
				b.WriteString(s.collectionResponse(sc, h, tags[i]))
			}
		}
	}
	b.WriteString("</d:multistatus>")
	w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
	davHeaders(w)
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write([]byte(b.String()))
}

// tag is what the collection's server said of its state: getctag and
// sync-token, empty when it did not answer (S-21: the collection stays in
// the list without them, the answer is not a 502).
type tag struct{ ctag, sync string }

// ctags asks every collection for its state, FanoutParallelism at a time,
// each within the source timeout.
func (s *Server) ctags(ctx context.Context, list []scoped) []tag {
	out := make([]tag, len(list))
	sem := make(chan struct{}, conf.FanoutParallelism)
	var wg sync.WaitGroup
	for i, sc := range list {
		wg.Add(1)
		go func(i int, sc scoped) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			c, cancel := context.WithTimeout(ctx, s.cfg.ProgressSourceTimeout)
			defer cancel()
			client, err := dav.NewClient(s.guard, sc.conn.Server, sc.conn.Login, sc.conn.Password, sc.conn.CertFingerprint)
			if err != nil {
				return
			}
			ctagName := xml.Name{Space: "http://calendarserver.org/ns/", Local: "getctag"}
			syncName := xml.Name{Space: "DAV:", Local: "sync-token"}
			ms, err := client.PropFind(c, sc.col.Path, dav.DepthZero, []xml.Name{ctagName, syncName})
			if err != nil || len(ms.Responses) == 0 {
				s.log.Debug("davloom: collection did not answer", "account", sc.conn.ID, "error", err)
				return
			}
			props := ms.Responses[0].Props
			out[i] = tag{ctag: innerText(props[ctagName]), sync: innerText(props[syncName])}
		}(i, sc)
	}
	wg.Wait()
	return out
}

// innerText is the text of a property's inner XML.
func innerText(inner []byte) string {
	var v struct {
		Text string `xml:",chardata"`
	}
	if xml.Unmarshal([]byte("<x>"+string(inner)+"</x>"), &v) != nil {
		return ""
	}
	return strings.TrimSpace(v.Text)
}

var components = map[view.Kind]string{view.KindEvent: "VEVENT", view.KindTask: "VTODO", view.KindNote: "VJOURNAL"}

func (s *Server) collectionResponse(sc scoped, h home, t tag) string {
	name := sc.col.Name
	if name == "" {
		name = path.Base(strings.TrimSuffix(sc.col.Path, "/"))
	}
	types := "<d:collection/>"
	var extra strings.Builder
	switch h.path {
	case "/cal/":
		types += "<c:calendar/>"
		extra.WriteString("<c:supported-calendar-component-set>")
		for _, k := range sc.kinds {
			if comp := components[k]; comp != "" {
				extra.WriteString(`<c:comp name="` + comp + `"/>`)
			}
		}
		extra.WriteString("</c:supported-calendar-component-set>")
		if sc.col.Colour != "" {
			extra.WriteString("<ic:calendar-color>" + html.EscapeString(sc.col.Colour) + "</ic:calendar-color>")
		}
	case "/card/":
		types += "<card:addressbook/>"
	}
	priv := "<d:privilege><d:read/></d:privilege>"
	if !sc.col.ReadOnly {
		priv += "<d:privilege><d:write/></d:privilege><d:privilege><d:write-content/></d:privilege><d:privilege><d:bind/></d:privilege><d:privilege><d:unbind/></d:privilege>"
	}
	extra.WriteString("<d:current-user-privilege-set>" + priv + "</d:current-user-privilege-set>")
	if t.ctag != "" {
		extra.WriteString("<cs:getctag>" + html.EscapeString(t.ctag) + "</cs:getctag>")
	}
	if t.sync != "" {
		extra.WriteString("<d:sync-token>" + html.EscapeString(t.sync) + "</d:sync-token>")
	}
	return "<d:response><d:href>" + html.EscapeString(s.colHref(sc.conn.ID, sc.col.Path)) + "</d:href><d:propstat><d:prop>" +
		"<d:resourcetype>" + types + "</d:resourcetype><d:displayname>" + html.EscapeString(name) + "</d:displayname>" +
		extra.String() + "</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>"
}
