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
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
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

	// Clock is when the stand says an object was last modified. Nil is the
	// wall clock; a fixture freezes it so «changed» holds tomorrow.
	Clock func() time.Time

	// Drop names properties the server silently discards on PUT, in every
	// component, the way lossy servers do (S-7.1). Without it a PUT is kept
	// byte for byte as sent, as Radicale and Baikal keep it; with it the object
	// is kept as go-vcard or go-ical write it back.
	Drop []string

	// FilesIgnoreIfNoneMatch makes a PUT under FileRoot overwrite a taken
	// name, the way x/net/webdav does (SFTPGo): the header is dropped before
	// the file server sees it.
	FilesIgnoreIfNoneMatch bool

	// MoveRewritesUID gives a record moved into another address book a UID of
	// the server's own, the way SoGo does (docs/servers.md, SV-18). The name it
	// lands under is not what the record layer keys on; the UID is.
	MoveRewritesUID bool

	// HollowCardQuery answers every addressbook-query 207 with no responses
	// at all, filter or none, the way Google does (docs/servers.md, SV-26),
	// while PROPFIND and addressbook-multiget hand the same book over.
	HollowCardQuery bool

	// CreateDropsColour makes the calendar without the calendar-color the
	// MKCALENDAR carried and names it in a 404 propstat of the
	// mkcalendar-response, the way Xandikos does (docs/servers.md, SV-41).
	// A PROPPATCH right after sets the colour and keeps it.
	CreateDropsColour bool

	// NoChangeTag answers a collection's change tag with 404, the way a
	// server without getctag or sync-token does. A whole read of such a
	// collection cannot be skipped by its tag; the soft TTL of S-4.2 applies.
	NoChangeTag bool

	// CalendarTextMatchFindsNothing answers every calendar-query that carries
	// a text-match 207 with no responses — the same answer as for an honest
	// miss — the way Xandikos does for any value that is not matched whole
	// and has a space in it (docs/servers.md, SV-47). A calendar-query with
	// no text-match is served normally.
	CalendarTextMatchFindsNothing bool

	// CardTextMatchFindsNothing answers every addressbook-query that carries
	// a text-match 207 with no responses, while a query without one hands the
	// book over — what DAViCal does to Cyrillic in another case as soon as
	// EMAIL or TEL stands among the alternatives (docs/servers.md, SV-51).
	CardTextMatchFindsNothing bool

	// CreateIgnoresComponentSet makes every calendar hold every component
	// whatever the MKCALENDAR asked for, the way Xandikos does: it drops the
	// set at creation, refuses a PROPPATCH of it with 409 because the
	// property is protected, and reports all four — and it takes a VTODO
	// into a calendar asked to hold only events (docs/servers.md, SV-43).
	CreateIgnoresComponentSet bool

	// CreateKeepsFirstComponent makes a calendar hold only the first
	// component of the set the MKCALENDAR asked for, answering 201 and
	// saying nothing of the rest, and answers 500 to a set of VJOURNAL alone
	// — the way iCloud does (docs/servers.md, SV-52).
	CreateKeepsFirstComponent bool

	// RefuseUIDTextMatch answers 412 with an empty body to a calendar-query
	// that carries a text-match on UID, the way iCloud does (docs/servers.md,
	// SV-54). A calendar-query without one, a multiget and a GET are served
	// normally.
	RefuseUIDTextMatch bool

	// RefuseEventWithoutEnd answers 404 with an empty body to a PUT of a
	// VEVENT that has DTSTART and neither DTEND nor DURATION, timed or
	// all-day, the way iCloud does (docs/servers.md, SV-53), although RFC
	// 5545 §3.6.1 allows one.
	RefuseEventWithoutEnd bool

	// DropsZeroLengthEnd keeps an event whose DTEND is its DTSTART — the
	// same time, the same zone — without the DTEND, the way Google does
	// (docs/servers.md, SV-56). An end on a date, and an end later than the
	// start, are kept.
	DropsZeroLengthEnd bool

	// MarksThePreferred keeps a card's TYPE values in capitals and marks the
	// first phone, the first address and the first e-mail PREF, the way
	// Google does (docs/servers.md, SV-56).
	MarksThePreferred bool

	// OneUIDPerAccount answers 412 with an empty body to a PUT of a calendar
	// object whose UID another resource of the account holds, in any
	// calendar and under any name, the way iCloud does (docs/servers.md,
	// SV-55). A PUT to the resource that holds the UID is an update and is
	// served.
	OneUIDPerAccount bool

	// BreaksOnTextMatch answers 500 to an addressbook-query that carries a
	// text-match, the way Xandikos does over a book holding one value with a
	// character outside ASCII (docs/servers.md, SV-42). Its own crash, not a
	// refusal: the same book without a filter hands every card over.
	BreaksOnTextMatch bool

	// DeafToMatchType answers an addressbook-query that carries a
	// match-type 207 with no responses — the same answer as for a card that
	// is not there — the way Xandikos does (docs/servers.md, SV-40). The
	// same filter without match-type is served normally.
	DeafToMatchType bool

	// RefuseAllPropInAddressData answers 400 to a REPORT whose address-data
	// holds an allprop element, the way Google does: RFC 6352 §10.4 does not
	// define one there, and an empty address-data is how the whole card is
	// asked for (docs/servers.md, SV-26).
	RefuseAllPropInAddressData bool

	// CreateRewritesUID writes a UID of the server's own over the one a new
	// card was sent with, the way Google does (docs/servers.md, SV-25). Google
	// names the resource itself as well; the fake keeps the address, because
	// the address is not what a record is found by — the UID is.
	CreateRewritesUID bool

	// CreateHidesTheRecord takes a new card and then shows it neither at the
	// address it was written to nor under any UID. No server seen does this;
	// Carrel's last answer to a record it cannot find again needs a guard all
	// the same (S-7.1).
	CreateHidesTheRecord bool

	// SearchIgnoresFilter answers a calendar-query with every object of the
	// collection, whatever the text-match said, the way SoGo does
	// (docs/servers.md, SV-16).
	SearchIgnoresFilter bool

	// RefuseCardQuery answers 403 with the supported-report precondition to an
	// addressbook-query, the way Yandex does — it has no such report at all,
	// while PROPFIND and addressbook-multiget both work (docs/servers.md,
	// SV-23).
	RefuseCardQuery bool

	// CreateAssignsItsOwnAddress answers 201 to MKCALENDAR and puts the
	// calendar at an address of its own, the way Yandex does (docs/servers.md,
	// SV-19): nothing is left at the address that was asked for, and the home
	// holds one child it did not hold before.
	CreateAssignsItsOwnAddress bool

	// RefuseUnsizedPUT answers 400 to a PUT that carries no Content-Length,
	// the way OpenCloud behind Apache does past a few kilobytes
	// (docs/servers.md, SV-15). It is how the rule "the length goes with the
	// body when it is known, and a refusal is said out loud otherwise" is held
	// without a network.
	RefuseUnsizedPUT bool

	// RefuseCollation answers 400 to a REPORT that names a collation, the way
	// sabre/dav answers one it does not know. Without it i;unicode-casemap is
	// honoured: go-webdav's own text-match counts case and ignores collation.
	RefuseCollation bool

	// ConvertsCards answers a read of a card that does not ask for vCard 4.0
	// in 3.0, without KIND and with sabre's PRODID, the way sabre/dav does
	// (Baikal, Nextcloud; docs/servers.md, SV-1): a GET whose Accept names
	// no version=4.0, a REPORT whose address-data names no version="4.0".
	// The card is kept as sent; only the answer is converted.
	ConvertsCards bool

	// CutPhotoDataURI keeps a card's PHOTO data: URI the way Radicale does
	// through vobject: cut at its comma, with ';' escaped — the image is gone.
	CutPhotoDataURI bool

	// ScheduleOutbox is the principal's schedule-outbox-URL, the mark of a
	// server that schedules (RFC 6638 §2.1): Nextcloud, Baikal, SoGo,
	// Yandex and Google announce one. Empty, the principal answers the
	// property 404, as Radicale and Xandikos do.
	ScheduleOutbox string

	// UserAddresses are the principal's calendar-user-address-set, as the
	// server writes them: mailto: addresses and, often, the principal's
	// own path beside them (RFC 6638 §2.4.1).
	UserAddresses []string

	mu        sync.Mutex
	sent      map[string][]byte // the body of the PUT being handled, by path
	calendars map[string]caldav.Calendar
	books     map[string]carddav.AddressBook
	colours   map[string]string
	trashed   map[string]string // collection -> the time the server says it was deleted
	ctags     map[string]int
	objects   map[string]object
	refuse    map[string]refusal
	stalls    map[string]chan struct{}
	hangUp    map[string]bool
	requests  []string
	seq       int
	// uidMatchesRefused counts the calendar-queries RefuseUIDTextMatch
	// answered 412.
	uidMatchesRefused int
	// uidsTaken counts the PUTs OneUIDPerAccount answered 412.
	uidsTaken int
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
		trashed:   make(map[string]string),
		ctags:     make(map[string]int),
		objects:   make(map[string]object),
		sent:      make(map[string][]byte),
		refuse:    make(map[string]refusal),
		stalls:    make(map[string]chan struct{}),
		hangUp:    make(map[string]bool),
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

// SetModified sets when a file under FileRoot was last changed, as the
// server reports it (getlastmodified).
func (s *Server) SetModified(p string, at time.Time) {
	if err := os.Chtimes(s.localPath(p), at, at); err != nil {
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

// Names lists the entries of a folder under FileRoot, as the server holds
// them; nil for a folder that is not there.
func (s *Server) Names(p string) []string {
	entries, err := os.ReadDir(s.localPath(p))
	if err != nil {
		return nil
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
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

// Trash puts a collection in the server's own trash bin the way Nextcloud
// does (docs/servers.md, SV-30): it stays at its address, stays among the
// children of the home, and still calls itself a calendar — only a request
// that asks about the deletion is told, through nc:deleted-at.
func (s *Server) Trash(collection, when string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trashed[collectionKey(collection)] = when
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

// UIDMatchesRefused is how many calendar-queries with a text-match on UID
// RefuseUIDTextMatch has refused so far.
func (s *Server) UIDMatchesRefused() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.uidMatchesRefused
}

// UIDsTaken is how many PUTs OneUIDPerAccount has refused so far.
func (s *Server) UIDsTaken() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.uidsTaken
}

// uidHeldElsewhere reports a calendar body whose UID another resource under
// the calendar home holds (SV-55).
func (s *Server) uidHeldElsewhere(p string, body []byte) bool {
	uid := calendarUID(body)
	if uid == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for at, o := range s.objects {
		if at != p && strings.HasPrefix(at, CalendarHome) && calendarUID(o.data) == uid {
			return true
		}
	}
	return false
}

// calendarUID is the UID of the first component of a calendar body that has
// one, or "".
func calendarUID(body []byte) string {
	cal, err := ical.NewDecoder(bytes.NewReader(body)).Decode()
	if err != nil {
		return ""
	}
	for _, c := range cal.Children {
		if prop := c.Props.Get(ical.PropUID); prop != nil && prop.Value != "" {
			return prop.Value
		}
	}
	return ""
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

func (s *Server) serveInner(w http.ResponseWriter, r *http.Request, cal, card, files http.Handler) {
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
	if s.RefuseAllPropInAddressData && r.Method == "REPORT" && bytes.Contains(body, []byte("address-data")) &&
		bytes.Contains(body, []byte("allprop")) {
		http.Error(w, `{"error":{"code":400,"status":"INVALID_ARGUMENT"}}`, http.StatusBadRequest)
		return
	}
	if s.CalendarTextMatchFindsNothing && r.Method == "REPORT" && bytes.Contains(body, []byte("calendar-query")) &&
		bytes.Contains(body, []byte("text-match")) {
		w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><D:multistatus xmlns:D="DAV:"/>`)
		return
	}
	if s.CardTextMatchFindsNothing && r.Method == "REPORT" && bytes.Contains(body, []byte("addressbook-query")) &&
		bytes.Contains(body, []byte("text-match")) {
		w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><D:multistatus xmlns:D="DAV:"/>`)
		return
	}
	if s.RefuseUIDTextMatch && r.Method == "REPORT" && bytes.Contains(body, []byte("calendar-query")) &&
		bytes.Contains(body, []byte("text-match")) && bytes.Contains(body, []byte(`name="UID"`)) {
		s.mu.Lock()
		s.uidMatchesRefused++
		s.mu.Unlock()
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	}
	if s.RefuseEventWithoutEnd && r.Method == http.MethodPut && strings.HasPrefix(p, CalendarHome) && eventWithoutEnd(body) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if s.OneUIDPerAccount && r.Method == http.MethodPut && strings.HasPrefix(p, CalendarHome) && s.uidHeldElsewhere(p, body) {
		s.mu.Lock()
		s.uidsTaken++
		s.mu.Unlock()
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	}
	if s.BreaksOnTextMatch && r.Method == "REPORT" && bytes.Contains(body, []byte("addressbook-query")) &&
		bytes.Contains(body, []byte("text-match")) {
		http.Error(w, "UnicodeEncodeError: 'ascii' codec can't encode characters", http.StatusInternalServerError)
		return
	}
	if s.DeafToMatchType && r.Method == "REPORT" && bytes.Contains(body, []byte("addressbook-query")) &&
		bytes.Contains(body, []byte("match-type")) {
		w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><D:multistatus xmlns:D="DAV:"/>`)
		return
	}
	if s.HollowCardQuery && r.Method == "REPORT" && bytes.Contains(body, []byte("addressbook-query")) {
		w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><D:multistatus xmlns:D="DAV:"/>`)
		return
	}
	if s.RefuseCardQuery && r.Method == "REPORT" && bytes.Contains(body, []byte("addressbook-query")) {
		w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
		http.Error(w, `<?xml version="1.0" encoding="UTF-8"?><D:error xmlns:D="DAV:"><supported-report xmlns="DAV:"/></D:error>`, http.StatusForbidden)
		return
	}
	if r.Method == "REPORT" && bytes.Contains(body, []byte("collation=")) {
		if s.RefuseCollation {
			http.Error(w, "Collation type is not supported", http.StatusBadRequest)
			return
		}
		if bytes.Contains(body, []byte(`"i;unicode-casemap"`)) {
			r = r.WithContext(context.WithValue(r.Context(), foldCase{}, true))
		}
	}
	if s.ConvertsCards && strings.HasPrefix(p, AddressBookHome) && !asksVCard4(r, body) {
		r = r.WithContext(context.WithValue(r.Context(), inVCard3{}, true))
	}
	if r.Method == http.MethodPut {
		s.mu.Lock()
		s.sent[p] = body
		s.mu.Unlock()
	}

	// go-webdav's server knows neither MKCALENDAR nor PROPPATCH on a
	// collection, lets MKCOL overwrite one that exists, and answers 501 to
	// MOVE of a record.
	switch r.Method {
	case "MOVE":
		if !strings.HasPrefix(p, FileRoot) {
			s.move(w, r)
			return
		}
	case http.MethodDelete:
		// go-webdav's CalDAV backend takes a calendar's address for a record
		// and answers 404; a server deletes the collection with what it holds.
		if s.hasCollection(p) {
			s.RemoveCollection(p)
			w.WriteHeader(http.StatusNoContent)
			return
		}
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

	if r.Method == "PROPFIND" && p == Principal && bytes.Contains(body, []byte("schedule-outbox-URL")) {
		s.principalProps(w)
		return
	}
	if r.Method == "PROPFIND" && (p == CalendarHome || p == AddressBookHome) && bytes.Contains(body, []byte("getctag")) {
		s.homeProps(w, p)
		return
	}
	if r.Method == "PROPFIND" && r.Header.Get("Depth") == "0" && s.hasCollection(p) && bytes.Contains(body, []byte("getctag")) {
		s.collectionTag(w, p)
		return
	}
	if r.Method == "PROPFIND" && r.Header.Get("Depth") == "0" && s.hasCollection(p) && bytes.Contains(body, []byte("calendar-color")) {
		s.collectionProps(w, p)
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
	// docs/servers.md, SV-15: a body whose length the client did not declare is
	// refused. ContentLength is -1 exactly when the request went out chunked.
	if r.Method == http.MethodPut && s.RefuseUnsizedPUT && strings.HasPrefix(p, FileRoot) && r.ContentLength < 0 {
		http.Error(w, "Content-Length required", http.StatusBadRequest)
		return
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

// principalProps answers the depth-0 PROPFIND of the principal for the two
// properties of RFC 6638 go-webdav's server does not know: what it announces
// is in the 200 propstat, what it does not in the 404 one.
func (s *Server) principalProps(w http.ResponseWriter) {
	var found, missing strings.Builder
	if s.ScheduleOutbox != "" {
		found.WriteString(`<C:schedule-outbox-URL><D:href>` + escape(s.ScheduleOutbox) + `</D:href></C:schedule-outbox-URL>`)
	} else {
		missing.WriteString(`<C:schedule-outbox-URL/>`)
	}
	if len(s.UserAddresses) > 0 {
		found.WriteString(`<C:calendar-user-address-set>`)
		for _, a := range s.UserAddresses {
			found.WriteString(`<D:href>` + escape(a) + `</D:href>`)
		}
		found.WriteString(`</C:calendar-user-address-set>`)
	} else {
		missing.WriteString(`<C:calendar-user-address-set/>`)
	}
	var b strings.Builder
	b.WriteString(`<D:response><D:href>` + Principal + `</D:href>`)
	if found.Len() > 0 {
		b.WriteString(`<D:propstat><D:prop>` + found.String() + `</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>`)
	}
	if missing.Len() > 0 {
		b.WriteString(`<D:propstat><D:prop>` + missing.String() + `</D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>`)
	}
	b.WriteString(`</D:response>`)
	w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>`+
		`<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`+b.String()+`</D:multistatus>`)
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
		if when, trashed := s.trashed[p]; trashed {
			b.WriteString(`<NC:deleted-at>` + escape(when) + `</NC:deleted-at>`)
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
		`<D:multistatus xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/" xmlns:I="http://apple.com/ns/ical/" `+
		`xmlns:NC="http://nextcloud.com/ns">`+b.String()+`</D:multistatus>`)
}

// collectionProps answers the depth-0 PROPFIND of one collection for its
// displayname and calendar-color, as Radicale and sabre/dav do; go-webdav's
// server does not know calendar-color.
func (s *Server) collectionProps(w http.ResponseWriter, p string) {
	key := collectionKey(p)
	s.mu.Lock()
	name := s.calendars[key].Name
	if book, ok := s.books[key]; ok {
		name = book.Name
	}
	colour, hasColour := s.colours[key]
	s.mu.Unlock()
	var b strings.Builder
	b.WriteString(`<D:response><D:href>` + escape(key) + `</D:href><D:propstat><D:prop>`)
	b.WriteString(`<D:displayname>` + escape(name) + `</D:displayname>`)
	if hasColour {
		b.WriteString(`<I:calendar-color>` + escape(colour) + `</I:calendar-color>`)
	}
	b.WriteString(`</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>`)
	if !hasColour {
		b.WriteString(`<D:propstat><D:prop><I:calendar-color/></D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>`)
	}
	b.WriteString(`</D:response>`)
	w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>`+
		`<D:multistatus xmlns:D="DAV:" xmlns:I="http://apple.com/ns/ical/">`+b.String()+`</D:multistatus>`)
}

// collectionTag answers a depth-0 PROPFIND for a collection's change tag: the
// question a whole read asks before it reads (S-4.2). The tag moves with every
// write, Carrel's or a seed's. NoChangeTag answers the way a server without
// one does: the property named, and 404.
func (s *Server) collectionTag(w http.ResponseWriter, p string) {
	key := collectionKey(p)
	s.mu.Lock()
	tag := fmt.Sprintf("ctag-%d", s.ctags[key])
	none := s.NoChangeTag
	s.mu.Unlock()
	var b strings.Builder
	b.WriteString(`<D:response><D:href>` + escape(key) + `</D:href>`)
	if none {
		b.WriteString(`<D:propstat><D:prop><CS:getctag/><D:sync-token/></D:prop><D:status>HTTP/1.1 404 Not Found</D:status></D:propstat>`)
	} else {
		b.WriteString(`<D:propstat><D:prop><CS:getctag>` + tag + `</CS:getctag></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat>`)
	}
	b.WriteString(`</D:response>`)
	w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="utf-8"?>`+
		`<D:multistatus xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">`+b.String()+`</D:multistatus>`)
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
	if s.CreateIgnoresComponentSet {
		comps = []string{"VEVENT", "VTODO", "VJOURNAL", "VFREEBUSY"}
	}
	if s.CreateKeepsFirstComponent && len(comps) > 0 {
		// docs/servers.md, SV-52: notes are not kept at all, and of any
		// other set only the first component is, without a word.
		if len(comps) == 1 && comps[0] == "VJOURNAL" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		comps = comps[:1]
	}
	if s.CreateAssignsItsOwnAddress {
		// docs/servers.md, SV-19: nothing is left at the address that was
		// asked for; the calendar is at one the server picked itself.
		s.mu.Lock()
		s.seq++
		key = CalendarHome + fmt.Sprintf("events-%d/", s.seq)
		s.mu.Unlock()
	}
	s.AddCalendar(caldav.Calendar{Path: key, Name: req.Prop.Name, SupportedComponentSet: comps})
	colour := strings.TrimSpace(req.Prop.Colour)
	if colour != "" && !s.CreateDropsColour {
		s.SetColour(key, colour)
	}
	if colour != "" && s.CreateDropsColour {
		// Made, and the colour is not: the answer says so per property,
		// and the status is still 201.
		w.Header().Set("Content-Type", `application/xml; charset="utf-8"`)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>`+
			`<C:mkcalendar-response xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:" xmlns:I="http://apple.com/ns/ical/">`+
			`<D:propstat><D:status>HTTP/1.1 200 OK</D:status><D:prop><D:displayname/></D:prop></D:propstat>`+
			`<D:propstat><D:status>HTTP/1.1 404 Not Found</D:status><D:prop><I:calendar-color/></D:prop></D:propstat>`+
			`</C:mkcalendar-response>`)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// eventWithoutEnd reports a calendar body holding a VEVENT with DTSTART and
// neither DTEND nor DURATION (docs/servers.md, SV-53).
func eventWithoutEnd(body []byte) bool {
	cal, err := ical.NewDecoder(bytes.NewReader(body)).Decode()
	if err != nil {
		return false
	}
	for _, c := range cal.Children {
		if c.Name == ical.CompEvent && c.Props.Get(ical.PropDateTimeStart) != nil &&
			c.Props.Get(ical.PropDateTimeEnd) == nil && c.Props.Get(ical.PropDuration) == nil {
			return true
		}
	}
	return false
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

// now is the stand's own clock: when it says an object was last modified.
// A test that freezes it gets the same answer tomorrow.
func (s *Server) now() time.Time {
	if s.Clock == nil {
		return time.Now()
	}
	return s.Clock()
}

func (s *Server) storeLocked(p string, data []byte) string {
	s.seq++
	etag := fmt.Sprintf("e%d", s.seq)
	s.objects[p] = object{data: data, etag: etag, mod: s.now()}
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
	if _, isBook := s.books[parent(p)]; isBook && !exists {
		switch {
		case s.CreateHidesTheRecord:
			s.seq++
			return object{etag: fmt.Sprintf("e%d", s.seq), mod: s.now()}, nil
		case s.CreateRewritesUID:
			s.seq++
			data = rewriteUID(data, fmt.Sprintf("sv25-%d", s.seq))
		}
	}
	s.storeLocked(p, data)
	return s.objects[p], nil
}

// move moves one record to another collection of this server, the way
// Radicale and sabre/dav do: the body and its version travel as they are, a
// taken name with Overwrite: F is 412, and a destination outside a collection
// is 409 (RFC 4918 §9.9.4).
func (s *Server) move(w http.ResponseWriter, r *http.Request) {
	dest, err := url.Parse(r.Header.Get("Destination"))
	if err != nil || dest.Path == "" {
		http.Error(w, "bad Destination", http.StatusBadRequest)
		return
	}
	src, dst := r.URL.Path, dest.Path
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.objects[src]
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, isCal := s.calendars[parent(dst)]
	_, isBook := s.books[parent(dst)]
	if !isCal && !isBook {
		http.Error(w, "no collection at the destination", http.StatusConflict)
		return
	}
	if _, taken := s.objects[dst]; taken {
		if r.Header.Get("Overwrite") != "T" {
			http.Error(w, "the destination exists", http.StatusPreconditionFailed)
			return
		}
	}
	delete(s.objects, src)
	if s.MoveRewritesUID && isBook {
		s.seq++
		s.storeLocked(dst, rewriteUID(o.data, fmt.Sprintf("sv18-%d", s.seq)))
	} else {
		s.objects[dst] = o
	}
	s.ctags[parent(src)]++
	s.ctags[parent(dst)]++
	w.WriteHeader(http.StatusCreated)
}

// rewriteUID puts another identifier in a record, leaving everything else as
// it was — what SV-18 does to a card on its way to another book, and SV-25
// to one on its way in.
func rewriteUID(data []byte, uid string) []byte {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "UID:") {
			continue
		}
		lines[i] = "UID:" + uid
		if strings.HasSuffix(line, "\r") {
			lines[i] += "\r"
		}
	}
	return []byte(strings.Join(lines, "\n"))
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
	// go-webdav serves the object as go-ical writes it back, not as stored:
	// a folded line comes out whole. The length it announces is of what it
	// serves, or a client reading the declared length meets an early end.
	var served bytes.Buffer
	if err := ical.NewEncoder(&served).Encode(data); err != nil {
		return nil, err
	}
	return &caldav.CalendarObject{
		Path: p, ETag: o.etag, ModTime: o.mod, ContentLength: int64(served.Len()), Data: data,
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

// foldCase marks a REPORT that asked for i;unicode-casemap.
type foldCase struct{}

func (b calBackend) QueryCalendarObjects(ctx context.Context, p string, q *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	all, err := b.ListCalendarObjects(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	if b.s.SearchIgnoresFilter {
		// docs/servers.md, SV-16: SoGo answers a text-match with the whole
		// collection, so the search has to happen on this side.
		return all, nil
	}
	if ctx.Value(foldCase{}) == nil {
		return caldav.Filter(q, all)
	}
	// go-webdav's text-match counts case. Under i;unicode-casemap the query and
	// a copy of each object are folded to lower case in the matched properties,
	// and the objects whose copies match come back as stored.
	names := map[string]bool{}
	folded := *q
	folded.CompFilter = foldFilter(q.CompFilter, names)
	byPath := make(map[string]caldav.CalendarObject, len(all))
	copies := make([]caldav.CalendarObject, 0, len(all))
	for _, co := range all {
		byPath[co.Path] = co
		o, err := b.s.get(co.Path)
		if err != nil {
			return nil, err
		}
		c, err := calendarObject(co.Path, o)
		if err != nil {
			return nil, err
		}
		foldComponent(c.Data.Component, names)
		copies = append(copies, *c)
	}
	matched, err := caldav.Filter(&folded, copies)
	if err != nil {
		return nil, err
	}
	out := make([]caldav.CalendarObject, 0, len(matched))
	for _, m := range matched {
		out = append(out, byPath[m.Path])
	}
	return out, nil
}

func foldFilter(f caldav.CompFilter, names map[string]bool) caldav.CompFilter {
	out := f
	out.Props = make([]caldav.PropFilter, len(f.Props))
	for i, pf := range f.Props {
		if pf.TextMatch != nil {
			tm := *pf.TextMatch
			tm.Text = strings.ToLower(tm.Text)
			pf.TextMatch = &tm
			names[pf.Name] = true
		}
		out.Props[i] = pf
	}
	out.Comps = make([]caldav.CompFilter, len(f.Comps))
	for i, c := range f.Comps {
		out.Comps[i] = foldFilter(c, names)
	}
	return out
}

func foldComponent(c *ical.Component, names map[string]bool) {
	for name, props := range c.Props {
		if !names[name] {
			continue
		}
		for i := range props {
			props[i].Value = strings.ToLower(props[i].Value)
		}
	}
	for _, child := range c.Children {
		foldComponent(child, names)
	}
}

func dropComponentProps(c *ical.Component, names []string) {
	for _, n := range names {
		c.Props.Del(n)
	}
	for _, child := range c.Children {
		dropComponentProps(child, names)
	}
}

// dropZeroLengthEnds takes DTEND out of every event whose DTEND is its
// DTSTART to the letter — value and parameters — and says whether it took
// any (SV-56).
func dropZeroLengthEnds(c *ical.Component) bool {
	dropped := false
	for _, child := range c.Children {
		if child.Name != ical.CompEvent {
			continue
		}
		start, end := child.Props.Get(ical.PropDateTimeStart), child.Props.Get(ical.PropDateTimeEnd)
		if start == nil || end == nil || start.Value != end.Value || !strings.Contains(start.Value, "T") ||
			start.Params.Get(ical.ParamTimezoneID) != end.Params.Get(ical.ParamTimezoneID) {
			continue
		}
		child.Props.Del(ical.PropDateTimeEnd)
		dropped = true
	}
	return dropped
}

func (b calBackend) PutCalendarObject(_ context.Context, p string, cal *ical.Calendar, opts *caldav.PutCalendarObjectOptions) (*caldav.CalendarObject, error) {
	data := b.s.takeSent(p)
	zeroEnd := b.s.DropsZeroLengthEnd && dropZeroLengthEnds(cal.Component)
	if len(b.s.Drop) > 0 || data == nil || zeroEnd {
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
	// As with a calendar object: the length of what go-vcard writes back.
	var served bytes.Buffer
	if err := vcard.NewEncoder(&served).Encode(card); err != nil {
		return nil, err
	}
	return &carddav.AddressObject{
		Path: p, ETag: o.etag, ModTime: o.mod, ContentLength: int64(served.Len()), Card: card,
	}, nil
}

func (b cardBackend) GetAddressObject(ctx context.Context, p string, _ *carddav.AddressDataRequest) (*carddav.AddressObject, error) {
	o, err := b.s.get(p)
	if err != nil {
		return nil, err
	}
	return addressObject(p, converted(ctx, o))
}

// inVCard3 marks a read of a card that ConvertsCards answers in 3.0.
type inVCard3 struct{}

// asksVCard4 reports a read that names vCard 4.0: a GET by its Accept, a
// REPORT by its address-data.
func asksVCard4(r *http.Request, body []byte) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		return strings.Contains(strings.ReplaceAll(r.Header.Get("Accept"), `"`, ""), "version=4.0")
	case "REPORT":
		return bytes.Contains(body, []byte(`version="4.0"`))
	}
	return true
}

// converted is the card as sabre/dav answers a read that asked for no
// version: VERSION 3.0, sabre's PRODID, and KIND — which 3.0 does not hold —
// gone.
func converted(ctx context.Context, o object) object {
	if ctx.Value(inVCard3{}) == nil {
		return o
	}
	var out []string
	for _, l := range strings.Split(string(o.data), "\r\n") {
		name, _, _ := strings.Cut(l, ":")
		name, _, _ = strings.Cut(strings.ToUpper(name), ";")
		switch name {
		case "VERSION":
			out = append(out, "VERSION:3.0", "PRODID:-//Sabre//Sabre VObject 4.5.6//EN")
			continue
		case "KIND", "PRODID":
			continue
		}
		out = append(out, l)
	}
	o.data = []byte(strings.Join(out, "\r\n"))
	return o
}

func (b cardBackend) ListAddressObjects(ctx context.Context, p string, _ *carddav.AddressDataRequest) ([]carddav.AddressObject, error) {
	var out []carddav.AddressObject
	for op, o := range b.s.objectsIn(p) {
		ao, err := addressObject(op, converted(ctx, o))
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
	if b.s.CutPhotoDataURI && data != nil {
		data = cutPhotoDataURI(data)
	}
	if b.s.MarksThePreferred && data != nil {
		data = markPreferred(data)
	}
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

// cutPhotoDataURI is Radicale's handling of a PHOTO data: URI: the value ends
// at its first comma and ';' in what is left is escaped.
func cutPhotoDataURI(doc []byte) []byte {
	lines := strings.Split(string(doc), "\r\n")
	for i, l := range lines {
		name, value, ok := strings.Cut(l, ":")
		if !ok || !strings.HasPrefix(strings.ToUpper(name), "PHOTO") || !strings.HasPrefix(strings.ToLower(value), "data:") {
			continue
		}
		value, _, _ = strings.Cut(value, ",")
		lines[i] = name + ":" + strings.ReplaceAll(value, ";", `\;`)
	}
	return []byte(strings.Join(lines, "\r\n"))
}

// markPreferred is Google's keeping of a card's phones, addresses and
// e-mails: the TYPE values in capitals, and the first of each kind marked
// PREF — TEL;TYPE=cell is kept as TEL;TYPE=CELL,PREF, a bare EMAIL as
// EMAIL;TYPE=PREF (docs/servers.md, SV-56).
func markPreferred(doc []byte) []byte {
	lines := strings.Split(string(doc), "\r\n")
	seen := map[string]bool{}
	for i, l := range lines {
		head, value, ok := strings.Cut(l, ":")
		if !ok || strings.HasPrefix(l, " ") {
			continue
		}
		params := strings.Split(head, ";")
		name := strings.ToUpper(params[0][strings.LastIndex(params[0], ".")+1:])
		if name != "TEL" && name != "EMAIL" && name != "ADR" {
			continue
		}
		first := !seen[name]
		seen[name] = true
		typed := false
		for j, p := range params[1:] {
			k, v, _ := strings.Cut(p, "=")
			if !strings.EqualFold(k, "TYPE") {
				continue
			}
			v = strings.ToUpper(v)
			if first && !slices.Contains(strings.Split(v, ","), "PREF") {
				v += ",PREF"
			}
			params[j+1] = "TYPE=" + v
			typed = true
		}
		if first && !typed {
			params = append(params, "TYPE=PREF")
		}
		lines[i] = strings.Join(params, ";") + ":" + value
	}
	return []byte(strings.Join(lines, "\r\n"))
}

func (b cardBackend) DeleteAddressObject(_ context.Context, p string) error { return b.s.remove(p) }

// HangUpOnce makes the next request with this method do its work and then
// lose its connection before the answer: the fault of docs/servers.md SV-9
// seen from the server's side. The write lands, the client sees the socket
// close, and the guard — for a write on a connection the pool had kept —
// sends it again, where it meets the thing the first copy made.
//
// Nextcloud did exactly this twice in the full live run of 21.09.2026, to a
// PUT and to an MKCOL, and Carrel told the person nothing had been made.
func (s *Server) HangUpOnce(method string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hangUp[method] = true
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request, cal, card, files http.Handler) {
	s.mu.Lock()
	hang := s.hangUp[r.Method]
	if hang {
		delete(s.hangUp, r.Method)
	}
	s.mu.Unlock()
	if !hang {
		s.serveInner(w, r, cal, card, files)
		return
	}
	// The request is served into a recorder, so the server really does hold
	// what it was sent; then the socket closes with nothing written back.
	s.serveInner(httptest.NewRecorder(), r, cal, card, files)
	if hj, ok := w.(http.Hijacker); ok {
		if conn, _, err := hj.Hijack(); err == nil {
			_ = conn.Close()
		}
	}
}
