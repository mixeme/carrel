// Package davtest serves CalDAV and CardDAV from memory for tests. The
// protocol side is go-webdav's own server, so PROPFIND, REPORT and
// conditional PUT answer the way a server does, not the way a test author
// remembers one. Only what go-webdav leaves to the backend is written here:
// storage, preconditions, colour and change tag.
package davtest

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
)

// The layout go-webdav's handlers expect: depth decides the resource type.
const (
	Principal       = "/alice/"
	CalendarHome    = "/alice/calendars/"
	AddressBookHome = "/alice/contacts/"
	// FileRoot holds plain folders and files, served by go-webdav's own
	// WebDAV handler over a temporary directory.
	FileRoot = "/alice/files/"
)

// Server is one in-memory DAV account.
type Server struct {
	URL string

	// Login and Password, when Login is set, are required as HTTP Basic.
	Login, Password string

	// Drop names properties the server silently discards on PUT, in every
	// component, the way lossy servers do (S-7.1). Without it a PUT is kept
	// byte for byte as sent, as Radicale and Baikal keep it; with it the object
	// is kept as go-vcard or go-ical write it back.
	Drop []string

	// FilesIgnoreIfNoneMatch makes a PUT under FileRoot overwrite a taken
	// name, the way x/net/webdav does (SFTPGo): the header is dropped before
	// the file server sees it.
	FilesIgnoreIfNoneMatch bool

	mu        sync.Mutex
	sent      map[string][]byte // the body of the PUT being handled, by path
	calendars map[string]caldav.Calendar
	books     map[string]carddav.AddressBook
	colours   map[string]string
	ctags     map[string]int
	objects   map[string]object
	refuse    map[string]refusal
	stalls    map[string]chan struct{}
	requests  []string
	seq       int
	dir       string // the file tree behind FileRoot
}

type refusal struct {
	code int
	body string
}

type object struct {
	data []byte
	etag string
	mod  time.Time
}

// New starts a server that stops with the test.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{
		calendars: make(map[string]caldav.Calendar),
		books:     make(map[string]carddav.AddressBook),
		colours:   make(map[string]string),
		ctags:     make(map[string]int),
		objects:   make(map[string]object),
		sent:      make(map[string][]byte),
		refuse:    make(map[string]refusal),
		stalls:    make(map[string]chan struct{}),
		dir:       t.TempDir(),
	}
	if err := os.MkdirAll(s.localPath(FileRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	cal := &caldav.Handler{Backend: calBackend{s}}
	card := &carddav.Handler{Backend: cardBackend{s}}
	files := &webdav.Handler{FileSystem: webdav.LocalFileSystem(s.dir)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.serve(w, r, cal, card, files)
	}))
	t.Cleanup(srv.Close)
	// Cleanups run last first: a stalled request is let go before Close waits
	// for it.
	t.Cleanup(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for m, ch := range s.stalls {
			close(ch)
			delete(s.stalls, m)
		}
	})
	s.URL = srv.URL
	return s
}

func (s *Server) localPath(p string) string {
	return filepath.Join(s.dir, filepath.FromSlash(strings.TrimPrefix(p, "/")))
}

// SeedFile stores a file under FileRoot, creating its folders.
func (s *Server) SeedFile(p, body string) {
	local := s.localPath(p)
	if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(local, []byte(body), 0o644); err != nil {
		panic(err)
	}
}

// File returns a file under FileRoot as the server holds it.
func (s *Server) File(p string) (string, bool) {
	data, err := os.ReadFile(s.localPath(p))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// SeedFolder creates an empty folder under FileRoot.
func (s *Server) SeedFolder(p string) {
	if err := os.MkdirAll(s.localPath(p), 0o755); err != nil {
		panic(err)
	}
}

// Accept lifts a Refuse.
func (s *Server) Accept(method string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.refuse, method)
}

// Stall holds every request with the method until release is called or the
// client gives up, the way a server that does not answer holds a connection.
func (s *Server) Stall(method string) (release func()) {
	ch := make(chan struct{})
	s.mu.Lock()
	s.stalls[method] = ch
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.stalls[method] == ch {
			close(ch)
			delete(s.stalls, method)
		}
	}
}

// AddCalendar adds a calendar under CalendarHome. An empty component set
// means all three.
func (s *Server) AddCalendar(c caldav.Calendar) {
	c.Path = collectionKey(c.Path)
	if len(c.SupportedComponentSet) == 0 {
		c.SupportedComponentSet = []string{ical.CompEvent, ical.CompToDo, ical.CompJournal}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calendars[c.Path] = c
}

// AddAddressBook adds an address book under AddressBookHome.
func (s *Server) AddAddressBook(b carddav.AddressBook) {
	b.Path = collectionKey(b.Path)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.books[b.Path] = b
}

// SetColour sets calendar-color on a collection.
func (s *Server) SetColour(collection, colour string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.colours[collectionKey(collection)] = colour
}

// RemoveCollection deletes a calendar or an address book with everything in
// it, as another client would.
func (s *Server) RemoveCollection(p string) {
	p = collectionKey(p)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.calendars, p)
	delete(s.books, p)
	delete(s.colours, p)
	delete(s.ctags, p)
	for k := range s.objects {
		if strings.HasPrefix(k, p) {
			delete(s.objects, k)
		}
	}
}

// Refuse makes every request with the method answer code and body, the way a
// server answers an account without the right.
func (s *Server) Refuse(method string, code int, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refuse[method] = refusal{code: code, body: body}
}

// Colour returns the calendar-color stored on a collection.
func (s *Server) Colour(collection string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.colours[collectionKey(collection)]
}

// Calendar returns a calendar as the server holds it.
func (s *Server) Calendar(p string) (caldav.Calendar, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.calendars[collectionKey(p)]
	return c, ok
}

// AddressBook returns an address book as the server holds it.
func (s *Server) AddressBook(p string) (carddav.AddressBook, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.books[collectionKey(p)]
	return b, ok
}

func (s *Server) hasCollection(p string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, cal := s.calendars[collectionKey(p)]
	_, book := s.books[collectionKey(p)]
	return cal || book
}

// Seed stores a resource as another client would have written it and returns
// its new ETag. Bare LF line ends become CRLF.
func (s *Server) Seed(p, body string) string {
	if !strings.Contains(body, "\r\n") {
		body = strings.ReplaceAll(body, "\n", "\r\n")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storeLocked(p, []byte(body))
}

// Object returns the stored body and ETag of a resource.
func (s *Server) Object(p string) (body, etag string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.objects[p]
	return string(o.data), o.etag, ok
}

// Paths returns the stored resource paths under a collection, sorted.
func (s *Server) Paths(collection string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for p := range s.objects {
		if parent(p) == collectionKey(collection) {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// Requests returns "METHOD path" for every request so far.
func (s *Server) Requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

// CountRequests counts requests with the method whose path has the prefix.
func (s *Server) CountRequests(method, prefix string) int {
	var n int
	for _, r := range s.Requests() {
		m, p, _ := strings.Cut(r, " ")
		if m == method && strings.HasPrefix(p, prefix) {
			n++
		}
	}
	return n
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request, cal, card, files http.Handler) {
	s.mu.Lock()
	s.requests = append(s.requests, r.Method+" "+r.URL.Path)
	hold := s.stalls[r.Method]
	s.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-r.Context().Done():
			return
		}
	}

	if s.Login != "" {
		u, p, ok := r.BasicAuth()
		if !ok || u != s.Login || p != s.Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="davtest"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}
	s.mu.Lock()
	ref, refused := s.refuse[r.Method]
	s.mu.Unlock()
	if refused {
		w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
		w.WriteHeader(ref.code)
		_, _ = io.WriteString(w, ref.body)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/.well-known/") {
		http.NotFound(w, r)
		return
	}
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	p := r.URL.Path
	if r.Method == http.MethodPut {
		s.mu.Lock()
		s.sent[p] = body
		s.mu.Unlock()
	}

	// go-webdav's server knows neither MKCALENDAR nor PROPPATCH on a
	// collection, and lets MKCOL overwrite one that exists.
	switch r.Method {
	case "MKCALENDAR":
		s.mkcalendar(w, p, body)
		return
	case "MKCOL":
		if s.hasCollection(p) {
			http.Error(w, "a collection already exists here", http.StatusMethodNotAllowed)
			return
		}
	case "PROPPATCH":
		if s.hasCollection(p) {
			s.proppatch(w, p, body)
			return
		}
	}

	if r.Method == "PROPFIND" && (p == CalendarHome || p == AddressBookHome) && bytes.Contains(body, []byte("getctag")) {
		s.homeProps(w, p)
		return
	}
	// go-webdav hands a record's DELETE to the backend without its If-Match;
	// its file server checks the header itself.
	if r.Method == http.MethodDelete && !strings.HasPrefix(p, FileRoot) && !s.deletePrecondition(w, r) {
		return
	}
	if r.Method == http.MethodPut && s.FilesIgnoreIfNoneMatch && strings.HasPrefix(p, FileRoot) {
		r.Header.Del("If-None-Match")
	}

	switch {
	case strings.HasPrefix(p, FileRoot):
		files.ServeHTTP(w, r)
	case strings.HasPrefix(p, CalendarHome):
		cal.ServeHTTP(w, r)
	case strings.HasPrefix(p, AddressBookHome):
		card.ServeHTTP(w, r)
	case bytes.Contains(body, []byte("addressbook-home-set")):
		card.ServeHTTP(w, r)
	default:
		cal.ServeHTTP(w, r)
	}
}

func (s *Server) deletePrecondition(w http.ResponseWriter, r *http.Request) bool {
	h := webdav.ConditionalMatch(r.Header.Get("If-Match"))
	if !h.IsSet() {
		return true
	}
	s.mu.Lock()
	o, ok := s.objects[r.URL.Path]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return false
	}
	if match, err := h.MatchETag(o.etag); err != nil || !match {
		http.Error(w, "If-Match condition failed", http.StatusPreconditionFailed)
		return false
	}
	return true
}

// homeProps answers the depth-1 PROPFIND for calendar-color and getctag that
// go-webdav's server does not know.
func (s *Server) homeProps(w http.ResponseWriter, home string) {
	s.mu.Lock()
	var paths []string
	if home == CalendarHome {
		for p := range s.calendars {
			paths = append(paths, p)
		}
	} else {
		for p := range s.books {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	var b strings.Builder
	for _, p := range paths {
		readOnly := s.calendars[p].ReadOnly || s.books[p].ReadOnly
		b.WriteString(`<D:response><D:href>` + escape(p) + `</D:href><D:propstat><D:prop>`)
		b.WriteString(`<CS:getctag>` + fmt.Sprintf("ctag-%d", s.ctags[p]) + `</CS:getctag>`)
		b.WriteString(`<D:current-user-privilege-set><D:privilege><D:read/></D:privilege>`)
		if !readOnly {
			b.WriteString(`<D:privilege><D:write/></D:privilege>`)
		}
		b.WriteString(`</D:current-user-privilege-set>`)
		colour, hasColour := s.colours[p]
		if hasColour {
			b.WriteString(`<I:calendar-color>` + escape(colour) + `</I:calendar-color>`)
		}
		b.WriteString(`</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>`)
		if !hasColour {
			b.WriteString(`<D:propstat><D:prop><I:calendar-color/></D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>`)
		}
		b.WriteString(`</D:response>`)
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>`+
		`<D:multistatus xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/" xmlns:I="http://apple.com/ns/ical/">`+
		b.String()+`</D:multistatus>`)
}

type mkcalendarRequest struct {
	XMLName xml.Name
	Prop    struct {
		Name       string `xml:"DAV: displayname"`
		Colour     string `xml:"http://apple.com/ns/ical/ calendar-color"`
		Components struct {
			Comp []struct {
				Name string `xml:"name,attr"`
			} `xml:"urn:ietf:params:xml:ns:caldav comp"`
		} `xml:"urn:ietf:params:xml:ns:caldav supported-calendar-component-set"`
	} `xml:"DAV: set>prop"`
}

// mkcalendar creates a calendar directly in CalendarHome with its name,
// colour and component set (RFC 4791 §5.3.1).
func (s *Server) mkcalendar(w http.ResponseWriter, p string, body []byte) {
	key := collectionKey(p)
	if parent(strings.TrimSuffix(key, "/")) != CalendarHome {
		http.Error(w, "calendar creation not allowed here", http.StatusForbidden)
		return
	}
	if s.hasCollection(key) {
		http.Error(w, "a collection already exists here", http.StatusMethodNotAllowed)
		return
	}
	var req mkcalendarRequest
	if err := xml.Unmarshal(body, &req); err != nil || req.XMLName != (xml.Name{Space: "urn:ietf:params:xml:ns:caldav", Local: "mkcalendar"}) {
		http.Error(w, "not a mkcalendar body", http.StatusBadRequest)
		return
	}
	var comps []string
	for _, c := range req.Prop.Components.Comp {
		comps = append(comps, c.Name)
	}
	s.AddCalendar(caldav.Calendar{Path: key, Name: req.Prop.Name, SupportedComponentSet: comps})
	if c := strings.TrimSpace(req.Prop.Colour); c != "" {
		s.SetColour(key, c)
	}
	w.WriteHeader(http.StatusCreated)
}

type propertyUpdate struct {
	Set []struct {
		Props []struct {
			XMLName xml.Name
			Value   string `xml:",chardata"`
		} `xml:",any"`
	} `xml:"DAV: set>prop"`
}

// proppatch sets displayname and calendar-color on a collection. Anything
// else is refused, and then nothing is set: PROPPATCH is atomic (RFC 4918
// §9.2), and the others answer 424.
func (s *Server) proppatch(w http.ResponseWriter, p string, body []byte) {
	var pu propertyUpdate
	if err := xml.Unmarshal(body, &pu); err != nil {
		http.Error(w, "not a propertyupdate body", http.StatusBadRequest)
		return
	}
	displayName := xml.Name{Space: "DAV:", Local: "displayname"}
	colour := xml.Name{Space: "http://apple.com/ns/ical/", Local: "calendar-color"}
	type change struct {
		name  xml.Name
		value string
	}
	var changes []change
	refused := false
	for _, set := range pu.Set {
		for _, prop := range set.Props {
			changes = append(changes, change{prop.XMLName, prop.Value})
			refused = refused || (prop.XMLName != displayName && prop.XMLName != colour)
		}
	}
	key := collectionKey(p)
	s.mu.Lock()
	if !refused {
		for _, ch := range changes {
			switch ch.name {
			case displayName:
				if c, ok := s.calendars[key]; ok {
					c.Name = ch.value
					s.calendars[key] = c
				}
				if b, ok := s.books[key]; ok {
					b.Name = ch.value
					s.books[key] = b
				}
			case colour:
				s.colours[key] = strings.TrimSpace(ch.value)
			}
		}
		s.ctags[key]++
	}
	s.mu.Unlock()

	var b strings.Builder
	for _, ch := range changes {
		status := "200 OK"
		switch {
		case refused && ch.name != displayName && ch.name != colour:
			status = "403 Forbidden"
		case refused:
			status = "424 Failed Dependency"
		}
		b.WriteString(`<D:propstat><D:prop><X:` + escape(ch.name.Local) + ` xmlns:X="` + escape(ch.name.Space) + `"/></D:prop>`)
		b.WriteString(`<D:status>HTTP/1.1 ` + status + `</D:status></D:propstat>`)
	}
	w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>`+
		escape(key)+`</D:href>`+b.String()+`</D:response></D:multistatus>`)
}

// takeSent is the body of the PUT to p as it came, once.
func (s *Server) takeSent(p string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := s.sent[p]
	delete(s.sent, p)
	return data
}

func (s *Server) storeLocked(p string, data []byte) string {
	s.seq++
	etag := fmt.Sprintf("e%d", s.seq)
	s.objects[p] = object{data: data, etag: etag, mod: time.Now()}
	s.ctags[parent(p)]++
	return etag
}

func (s *Server) put(p string, data []byte, ifMatch, ifNoneMatch webdav.ConditionalMatch) (object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, exists := s.objects[p]
	if ifNoneMatch.IsSet() && exists {
		if match, _ := ifNoneMatch.MatchETag(cur.etag); match {
			return object{}, webdav.NewHTTPError(http.StatusPreconditionFailed, fmt.Errorf("If-None-Match condition failed"))
		}
	}
	if ifMatch.IsSet() {
		if !exists {
			return object{}, webdav.NewHTTPError(http.StatusPreconditionFailed, fmt.Errorf("If-Match: no such resource"))
		}
		if match, _ := ifMatch.MatchETag(cur.etag); !match {
			return object{}, webdav.NewHTTPError(http.StatusPreconditionFailed, fmt.Errorf("If-Match condition failed"))
		}
	}
	s.storeLocked(p, data)
	return s.objects[p], nil
}

func (s *Server) remove(p string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.objects[p]; !ok {
		return notFound()
	}
	delete(s.objects, p)
	s.ctags[parent(p)]++
	return nil
}

func (s *Server) objectsIn(collection string) map[string]object {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]object)
	for p, o := range s.objects {
		if parent(p) == collectionKey(collection) {
			out[p] = o
		}
	}
	return out
}

func (s *Server) get(p string) (object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.objects[p]
	if !ok {
		return object{}, notFound()
	}
	return o, nil
}

func collectionKey(p string) string { return strings.TrimSuffix(p, "/") + "/" }

func parent(p string) string { return collectionKey(path.Dir(p)) }

func notFound() error {
	return webdav.NewHTTPError(http.StatusNotFound, fmt.Errorf("not found"))
}

func escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

type calBackend struct{ s *Server }

func (b calBackend) CurrentUserPrincipal(context.Context) (string, error) { return Principal, nil }
func (b calBackend) CalendarHomeSetPath(context.Context) (string, error)  { return CalendarHome, nil }

func (b calBackend) CreateCalendar(_ context.Context, c *caldav.Calendar) error {
	b.s.AddCalendar(*c)
	return nil
}

func (b calBackend) ListCalendars(context.Context) ([]caldav.Calendar, error) {
	b.s.mu.Lock()
	defer b.s.mu.Unlock()
	var out []caldav.Calendar
	for _, c := range b.s.calendars {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (b calBackend) GetCalendar(_ context.Context, p string) (*caldav.Calendar, error) {
	b.s.mu.Lock()
	defer b.s.mu.Unlock()
	c, ok := b.s.calendars[collectionKey(p)]
	if !ok {
		return nil, notFound()
	}
	return &c, nil
}

func calendarObject(p string, o object) (*caldav.CalendarObject, error) {
	data, err := ical.NewDecoder(bytes.NewReader(o.data)).Decode()
	if err != nil {
		return nil, err
	}
	return &caldav.CalendarObject{
		Path: p, ETag: o.etag, ModTime: o.mod, ContentLength: int64(len(o.data)), Data: data,
	}, nil
}

func (b calBackend) GetCalendarObject(_ context.Context, p string, _ *caldav.CalendarCompRequest) (*caldav.CalendarObject, error) {
	o, err := b.s.get(p)
	if err != nil {
		return nil, err
	}
	return calendarObject(p, o)
}

func (b calBackend) ListCalendarObjects(_ context.Context, p string, _ *caldav.CalendarCompRequest) ([]caldav.CalendarObject, error) {
	var out []caldav.CalendarObject
	for op, o := range b.s.objectsIn(p) {
		co, err := calendarObject(op, o)
		if err != nil {
			return nil, err
		}
		out = append(out, *co)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (b calBackend) QueryCalendarObjects(ctx context.Context, p string, q *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	all, err := b.ListCalendarObjects(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	return caldav.Filter(q, all)
}

func dropComponentProps(c *ical.Component, names []string) {
	for _, n := range names {
		c.Props.Del(n)
	}
	for _, child := range c.Children {
		dropComponentProps(child, names)
	}
}

func (b calBackend) PutCalendarObject(_ context.Context, p string, cal *ical.Calendar, opts *caldav.PutCalendarObjectOptions) (*caldav.CalendarObject, error) {
	data := b.s.takeSent(p)
	if len(b.s.Drop) > 0 || data == nil {
		dropComponentProps(cal.Component, b.s.Drop)
		var buf bytes.Buffer
		if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
			return nil, webdav.NewHTTPError(http.StatusBadRequest, err)
		}
		data = buf.Bytes()
	}
	o, err := b.s.put(p, data, opts.IfMatch, opts.IfNoneMatch)
	if err != nil {
		return nil, err
	}
	return &caldav.CalendarObject{Path: p, ETag: o.etag, ModTime: o.mod}, nil
}

func (b calBackend) DeleteCalendarObject(_ context.Context, p string) error { return b.s.remove(p) }

type cardBackend struct{ s *Server }

func (b cardBackend) CurrentUserPrincipal(context.Context) (string, error) { return Principal, nil }
func (b cardBackend) AddressBookHomeSetPath(context.Context) (string, error) {
	return AddressBookHome, nil
}

func (b cardBackend) ListAddressBooks(context.Context) ([]carddav.AddressBook, error) {
	b.s.mu.Lock()
	defer b.s.mu.Unlock()
	var out []carddav.AddressBook
	for _, ab := range b.s.books {
		out = append(out, ab)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (b cardBackend) GetAddressBook(_ context.Context, p string) (*carddav.AddressBook, error) {
	b.s.mu.Lock()
	defer b.s.mu.Unlock()
	ab, ok := b.s.books[collectionKey(p)]
	if !ok {
		return nil, notFound()
	}
	return &ab, nil
}

func (b cardBackend) CreateAddressBook(_ context.Context, ab *carddav.AddressBook) error {
	b.s.AddAddressBook(*ab)
	return nil
}

func (b cardBackend) DeleteAddressBook(_ context.Context, p string) error {
	b.s.mu.Lock()
	defer b.s.mu.Unlock()
	delete(b.s.books, collectionKey(p))
	return nil
}

func addressObject(p string, o object) (*carddav.AddressObject, error) {
	card, err := vcard.NewDecoder(bytes.NewReader(o.data)).Decode()
	if err != nil {
		return nil, err
	}
	return &carddav.AddressObject{
		Path: p, ETag: o.etag, ModTime: o.mod, ContentLength: int64(len(o.data)), Card: card,
	}, nil
}

func (b cardBackend) GetAddressObject(_ context.Context, p string, _ *carddav.AddressDataRequest) (*carddav.AddressObject, error) {
	o, err := b.s.get(p)
	if err != nil {
		return nil, err
	}
	return addressObject(p, o)
}

func (b cardBackend) ListAddressObjects(_ context.Context, p string, _ *carddav.AddressDataRequest) ([]carddav.AddressObject, error) {
	var out []carddav.AddressObject
	for op, o := range b.s.objectsIn(p) {
		ao, err := addressObject(op, o)
		if err != nil {
			return nil, err
		}
		out = append(out, *ao)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (b cardBackend) QueryAddressObjects(ctx context.Context, p string, q *carddav.AddressBookQuery) ([]carddav.AddressObject, error) {
	all, err := b.ListAddressObjects(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	// RFC 6352 §10.5: a filter without prop-filters matches every card.
	// go-webdav's Match answers false there, which no real server does.
	if len(q.PropFilters) == 0 {
		return all, nil
	}
	return carddav.Filter(q, all)
}

func (b cardBackend) PutAddressObject(_ context.Context, p string, card vcard.Card, opts *carddav.PutAddressObjectOptions) (*carddav.AddressObject, error) {
	data := b.s.takeSent(p)
	if len(b.s.Drop) > 0 || data == nil {
		for _, n := range b.s.Drop {
			delete(card, strings.ToUpper(n))
		}
		var buf bytes.Buffer
		if err := vcard.NewEncoder(&buf).Encode(card); err != nil {
			return nil, webdav.NewHTTPError(http.StatusBadRequest, err)
		}
		data = buf.Bytes()
	}
	o, err := b.s.put(p, data, opts.IfMatch, opts.IfNoneMatch)
	if err != nil {
		return nil, err
	}
	return &carddav.AddressObject{Path: p, ETag: o.etag, ModTime: o.mod}, nil
}

func (b cardBackend) DeleteAddressObject(_ context.Context, p string) error { return b.s.remove(p) }
