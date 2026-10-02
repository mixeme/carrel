package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/publish"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// kindSpec ties a C-5.4 {kind} segment to the records it holds.
type kindSpec struct {
	segment   string
	kind      view.Kind
	component string // iCalendar main component; empty for contacts
	ext       string
	noun      string
}

var recordKinds = map[string]kindSpec{
	"contacts": {segment: "contacts", kind: view.KindContact, ext: ".vcf", noun: "contact"},
	"calendar": {segment: "calendar", kind: view.KindEvent, component: "VEVENT", ext: ".ics", noun: "event"},
	"tasks":    {segment: "tasks", kind: view.KindTask, component: "VTODO", ext: ".ics", noun: "task"},
	"notes":    {segment: "notes", kind: view.KindNote, component: "VJOURNAL", ext: ".ics", noun: "note"},
}

func segmentOf(k view.Kind) string {
	for seg, spec := range recordKinds {
		if spec.kind == k {
			return seg
		}
	}
	return ""
}

// origin is where a listed record lives: one catalog entry of one connection.
type origin struct {
	conn sealed.Connection
	col  sealed.Collection
}

func (o origin) listPath(segment string) string {
	return "/app/" + segment + "/" + o.conn.ID + "/" + url.PathEscape(o.col.Path)
}

func (o origin) recordPath(segment, uid string) string {
	return o.listPath(segment) + "/" + url.PathEscape(uid)
}

func (o origin) stamp() view.Stamp {
	return stamp(connLabel(o.conn), collectionLabel(o.col))
}

// listed is one record of a list with its origin. The rows of a combined view
// carry the origin (S-14); the rows of a single collection do not repeat it.
type listed struct {
	obj  *model.Object
	from origin
	// also are the other members of a linked group shown in this row (S-16).
	also []listed
	// prefer names, by property, the member whose value the linked row shows:
	// the answer the person gave when linking.
	prefer map[string]*model.Object
}

// pick is the record whose prop the row shows: the chosen member of a linked
// group, else the row's own record.
func (it listed) pick(prop string) *model.Object {
	if o := it.prefer[prop]; o != nil {
		return o
	}
	return it.obj
}

// listedUID is the identity of a listed record of kind k.
func listedUID(k view.Kind, it listed) string {
	if k == view.KindContact {
		return card.UID(it.obj)
	}
	return cal.UID(it.obj)
}

// collectionCtx is one owned collection opened for a request: the catalog
// entry, its connection, and a provider over a guarded client. A single
// collection is not a fanout (S-11): nothing here polls.
type collectionCtx struct {
	sess *session.Session
	spec kindSpec
	conn sealed.Connection
	col  sealed.Collection
	t    dav.Transport // for MOVE between collections of this connection (S-18)
	card *card.Provider
	cal  *cal.Provider
	tf   timeFmt
	// feed reads a subscription (S-25.2); nil for a collection of a server.
	feed *feedReader
	// ttl is the soft limit on a kept whole read where the server gives no
	// change tag (S-4.2, wholeread.go), and the clock it is measured by.
	ttl time.Duration
	now func() time.Time
}

// openCollection checks ownership and looks the collection up in the
// catalog. A collection that does not hold records of this kind answers 404,
// the same as a foreign one.
func (s *Server) openCollection(w http.ResponseWriter, r *http.Request, segment string, withUID bool) (*collectionCtx, bool) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return nil, false
	}
	spec, known := recordKinds[segment]
	if !known {
		s.notFound(w, r)
		return nil, false
	}
	account, col := r.PathValue("account"), r.PathValue("col")
	uid := ""
	if withUID {
		uid = r.PathValue("uid")
	}
	if !s.checkOwnership(w, r, sess, account, col, uid) {
		return nil, false
	}
	c, ok := s.collectionOf(sess, spec, account, col)
	if !ok || (c.feed != nil && !feedRoute(r)) {
		s.notFound(w, r)
		return nil, false
	}
	return c, true
}

// collectionOf opens a collection of the person's catalog that holds records
// of the kind, or reports that there is none: a foreign, gone-from-catalog or
// other-kind collection is the same "no".
func (s *Server) collectionOf(sess *session.Session, spec kindSpec, account, col string) (*collectionCtx, bool) {
	key, live := sess.DataKey()
	blob, opened := s.users.Catalog(sess.AccountID(), key)
	conn, hasConn := blob.Connection(account)
	entry, hasCol := blob.Collection(account, col)
	if !live || !opened || !hasConn || !hasCol || !entry.Has(spec.kind) {
		return nil, false
	}
	c, err := s.newCollectionCtx(sess, spec, conn, entry)
	return c, err == nil
}

// newCollectionCtx wires providers for a collection, in the catalog or not
// yet: a collection being made by a move joins the catalog last (S-12.3).
func (s *Server) newCollectionCtx(sess *session.Session, spec kindSpec, conn sealed.Connection, col sealed.Collection) (*collectionCtx, error) {
	endpoint := conn.Server
	if conn.Feed {
		// The providers of a feed only encode and parse; its records come
		// from the feed reader. Their client needs an address it can hold.
		endpoint, _ = publish.FeedURL(conn.Server)
	}
	client, err := dav.NewClient(s.guard, endpoint, conn.Login, conn.Password, conn.CertFingerprint)
	if err != nil {
		return nil, err
	}
	c := &collectionCtx{sess: sess, spec: spec, conn: conn, col: col, t: client, tf: s.timeFor(sess), ttl: s.cfg.CacheCollectionTTL, now: s.now}
	if conn.Feed {
		c.feed = s.newFeedReader(sess, conn)
	}
	if spec.kind == view.KindContact {
		c.card, err = card.New(client)
	} else {
		c.cal, err = cal.New(client)
	}
	if err != nil {
		return nil, err
	}
	if c.cal != nil && sess != nil && !conn.Feed {
		// A server that will not filter by UID is asked once per session
		// (docs/servers.md, SV-54), and the record is then found in a read
		// the session keeps (S-4.2).
		c.cal.Refusals = sessionRefusals{sess: sess, connID: conn.ID}
		c.cal.ReadWhole = c.readWhole
	}
	return c, nil
}

// sessionRefusals is cal.Refusals kept by the session, per connection.
type sessionRefusals struct {
	sess   *session.Session
	connID string
}

func (r sessionRefusals) Refused(collection, what string) bool {
	return r.sess.Refused(r.connID, collection, what)
}

func (r sessionRefusals) NoteRefused(collection, what string) {
	r.sess.NoteRefusal(r.connID, collection, what)
}

// readWhole is a component read whole, kept like any whole read (S-4.2): what
// a lookup the server would not filter by UID looks in (SV-54).
func (c *collectionCtx) readWhole(ctx context.Context, collection, component string) ([]*model.Object, error) {
	return cachedWholeRead(ctx, c.sess, c.t, c.conn.ID, collection, component, c.ttl, c.now(), calCodec,
		func() ([]*model.Object, bool, error) {
			got, bad, err := c.cal.Query(ctx, collection, component, nil, nil)
			return got, len(bad) == 0, err
		})
}

func (c *collectionCtx) origin() origin { return origin{conn: c.conn, col: c.col} }

func (c *collectionCtx) listPath() string { return c.origin().listPath(c.spec.segment) }

func (c *collectionCtx) recordPath(uid string) string {
	return c.origin().recordPath(c.spec.segment, uid)
}

// kindRecordPath is a record of another kind in the same calendar collection.
func (c *collectionCtx) kindRecordPath(k view.Kind, uid string) string {
	return c.origin().recordPath(segmentOf(k), uid)
}

func (c *collectionCtx) stamp() view.Stamp { return c.origin().stamp() }

// listed pairs the objects of this collection with it.
func (c *collectionCtx) listed(objs []*model.Object) []listed {
	out := make([]listed, len(objs))
	for i, o := range objs {
		out[i] = listed{obj: o, from: c.origin()}
	}
	return out
}

func (c *collectionCtx) source() view.SourceBlock {
	b := view.SourceBlock{Stamp: c.stamp(), Colour: catalogColour(c.col), ReadOnly: c.col.ReadOnly}
	b.LastRead, b.Changed = readingOf(c.sess, c.tf, c.conn.ID, c.col.Path)
	return b
}

// noteRead keeps when this list was read and, for a whole read, the newest
// change among its records (S-22). An agenda reads a week: it tells when, not
// what changed in the collection — the same rule as its count.
func (c *collectionCtx) noteRead(items []listed, whole bool) {
	noteRead(c.sess, c.conn.ID, c.col.Path, c.tf.now, items, whole)
}

func noteRead(sess *session.Session, accountID, path string, at time.Time, items []listed, whole bool) {
	r := session.Reading{At: at}
	if whole {
		for _, it := range items {
			if it.obj != nil && it.obj.Modified.After(r.Changed) {
				r.Changed = it.obj.Modified
			}
		}
	}
	sess.NoteReading(accountID, path, r)
}

// readingOf is LastRead and Changed of a collection (S-22), from what this
// session read; both empty when it has read no list of it.
func readingOf(sess *session.Session, tf timeFmt, accountID, path string) (lastRead, changed view.When) {
	r, ok := sess.ReadingOf(accountID, path)
	if !ok || r.At.IsZero() {
		// A feed fetched outside a list (a record, search) is measured,
		// not read as a list.
		return "", ""
	}
	lastRead = tf.formatWhen(r.At, true, tf.now)
	if !r.Changed.IsZero() {
		changed = view.When(tf.date(r.Changed) + ", " + tf.clock(r.Changed))
	}
	return lastRead, changed
}

func (c *collectionCtx) find(ctx context.Context, uid string) (*model.Object, error) {
	if c.feed != nil {
		return c.feed.find(ctx, c.spec.kind, uid)
	}
	if c.card != nil {
		return c.card.FindUID(ctx, c.col.Path, uid)
	}
	return c.cal.FindUID(ctx, c.col.Path, c.spec.component, uid)
}

// fetch reads one record by its address, whatever identity it holds: the way
// to a record a server named itself (S-7.1).
func (c *collectionCtx) fetch(ctx context.Context, path string) (*model.Object, error) {
	if c.feed != nil {
		return c.feed.find(ctx, c.spec.kind, strings.TrimPrefix(path, "feed/"))
	}
	if c.card != nil {
		return c.card.Get(ctx, path)
	}
	return c.cal.Get(ctx, path)
}

func (c *collectionCtx) update(ctx context.Context, obj *model.Object) error {
	var err error
	if c.card != nil {
		_, err = c.card.Update(ctx, obj)
	} else {
		_, err = c.cal.Update(ctx, obj)
	}
	return err
}

func (c *collectionCtx) create(ctx context.Context, path string, obj *model.Object) error {
	var err error
	if c.card != nil {
		_, err = c.card.Create(ctx, path, obj)
	} else {
		_, err = c.cal.Create(ctx, path, obj)
	}
	return err
}

func (c *collectionCtx) remove(ctx context.Context, path, etag string) error {
	if c.card != nil {
		return c.card.Delete(ctx, path, etag)
	}
	return c.cal.Delete(ctx, path, etag)
}

// text is a property value for display and for the form: vCard values arrive
// unescaped, iCalendar TEXT is unescaped here.
func (c *collectionCtx) text(obj *model.Object, prop string) string {
	if c.card != nil {
		return firstValue(obj, prop)
	}
	return cal.Text(obj, prop)
}

func (c *collectionCtx) body(obj *model.Object) ([]byte, error) {
	if c.card != nil {
		var buf bytes.Buffer
		if err := card.Encode(&buf, obj); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	return obj.CalendarDoc(), nil
}

func (c *collectionCtx) parse(path, etag string, body []byte) (*model.Object, error) {
	if c.card != nil {
		return card.Parse(path, etag, bytes.NewReader(body))
	}
	return cal.Parse(path, etag, bytes.NewReader(body))
}

func notFoundErr(err error) bool {
	return errors.Is(err, card.ErrNotFound) || errors.Is(err, cal.ErrNotFound)
}

// indexEntry is one record of a list this session loaded. RELATED-TO resolves
// against these entries and nothing else: no second REPORT, no other
// collection (S-11).
//
// Duplicates are found among these entries too, and only these (S-16): Keys
// are what the record is compared by and Detail the line that tells two
// members apart on the duplicates screen, both taken when the list was read.
type indexEntry struct {
	UID     string    `json:"uid"`
	Path    string    `json:"path"`
	ETag    string    `json:"etag"`
	Kind    view.Kind `json:"kind"`
	Title   string    `json:"title"`
	Related []string  `json:"related,omitempty"`
	Detail  string    `json:"detail,omitempty"`
	// At is the record's day, Unix seconds: when an event starts, when a
	// task is due, when a note was written. A note's is the order of the
	// notebook's list, which its neighbours follow (s-notes-20); every
	// kind's orders the list of Related to and names its day (s-tasks-10).
	At   int64      `json:"at,omitempty"`
	Keys dupes.Keys `json:"keys,omitzero"`
	// Fields are the single-valued properties a row shows, as the record
	// holds them: what the duplicates screen asks about when members
	// disagree, without reading a record again. PHOTO is a digest.
	Fields map[string]string `json:"fields,omitempty"`
}

// indexOf is the index of listed records of kind k; days are read in the
// zone of tf.
func indexOf(tf timeFmt, k view.Kind, items []listed) []indexEntry {
	out := make([]indexEntry, 0, len(items))
	for _, it := range items {
		o := it.obj
		e := indexEntry{Path: o.Path, ETag: o.ETag, Kind: k, Keys: dupes.KeysOf(k, o, tf.tz), Fields: choiceFields(k, o)}
		switch k {
		case view.KindContact:
			e.UID, e.Title = card.UID(o), contactName(o)
			e.Detail = strings.Join(nonEmpty(firstValue(o, "TEL"), firstValue(o, "EMAIL")), " · ")
		case view.KindEvent:
			e.UID, e.Title, e.Related = cal.UID(o), titleOr(cal.Text(o, "SUMMARY"), "Untitled event"), cal.RelatedTo(o)
			if tm, err := cal.TimingOf(o, tf.tz); err == nil && !tm.Start.IsZero() {
				e.Detail, e.At = tf.shortDue(tm), tm.Start.Unix()
			}
		case view.KindTask:
			e.UID, e.Title, e.Related = cal.UID(o), titleOr(cal.Text(o, "SUMMARY"), "Untitled task"), cal.RelatedTo(o)
			if due, err := cal.DueOf(o, tf.tz); err == nil && !due.Start.IsZero() {
				e.Detail, e.At = tf.shortDue(due), due.Start.Unix()
			}
		default:
			e.UID, e.Title, e.Related = cal.UID(o), noteTitle(cal.Text(o, "SUMMARY"), cal.Description(o)), cal.RelatedTo(o)
			if at := noteTime(o, tf.tz); !at.IsZero() {
				e.Detail, e.At = tf.shortDay(at), at.Unix()
			}
		}
		out = append(out, e)
	}
	return out
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func putIndex(sess *session.Session, accountID, path, component string, entries []indexEntry) {
	data, err := json.Marshal(entries)
	if err != nil {
		return
	}
	sess.Cache().PutETagMap(accountID, path+"\x00"+component, data)
}

func (c *collectionCtx) indexKey(component string) string {
	return c.col.Path + "\x00" + component
}

// components are the index slots of the collection: one per record kind a
// calendar collection can hold, one for an address book.
func (c *collectionCtx) components() []string {
	if c.card != nil {
		return []string{""}
	}
	return []string{"VEVENT", "VTODO", "VJOURNAL"}
}

func (c *collectionCtx) putIndex(entries []indexEntry) {
	putIndex(c.sess, c.conn.ID, c.col.Path, c.spec.component, entries)
}

// sessionIndex is every list this session has loaded, of every collection
// of the catalog. A RELATED-TO whose target is not in this collection still
// has a title when the target's own list was opened (review Р-12); the
// link, though, stays inside the collection — the address of a record is
// its collection's (S-11).
func (s *Server) sessionIndex(sess *session.Session, blob sealed.Blob) map[string]indexEntry {
	out := map[string]indexEntry{}
	eachIndexed(sess, blob, func(_ sealed.Collection, e indexEntry) {
		if _, seen := out[e.UID]; !seen {
			out[e.UID] = e
		}
	})
	return out
}

// indexSlots are the slots a collection's loaded lists are kept under: an
// address book's, then a calendar's events, tasks and notes. A collection
// fills either the first or the other three.
var indexSlots = []string{"", "VEVENT", "VTODO", "VJOURNAL"}

// eachIndexed walks every record of the lists this session has loaded, of
// every collection of the catalog that is not gone, in the catalog's order
// and then the order of indexSlots; no server is asked. Whoever needs
// records Carrel has already seen anywhere — the hints of the search box,
// the titles of links, the list of Related to, and later the people a note
// names (Mentions) — reads them here.
func eachIndexed(sess *session.Session, blob sealed.Blob, fn func(col sealed.Collection, e indexEntry)) {
	for _, col := range blob.Collections {
		if col.Gone {
			continue
		}
		for _, e := range indexedIn(sess, col) {
			fn(col, e)
		}
	}
}

// indexedIn is what this session has loaded of one collection: the entries
// of its lists in the order of indexSlots. A slot never loaded, or one that
// does not decode, gives nothing.
func indexedIn(sess *session.Session, col sealed.Collection) []indexEntry {
	var out []indexEntry
	for _, comp := range indexSlots {
		data, ok := sess.Cache().PeekETagMap(col.AccountID, col.Path+"\x00"+comp)
		if !ok {
			continue
		}
		var part []indexEntry
		if json.Unmarshal(data, &part) != nil {
			continue
		}
		out = append(out, part...)
	}
	return out
}

func (c *collectionCtx) index() []indexEntry {
	var out []indexEntry
	for _, comp := range c.components() {
		data, ok := c.sess.Cache().PeekETagMap(c.conn.ID, c.indexKey(comp))
		if !ok {
			continue
		}
		var part []indexEntry
		if json.Unmarshal(data, &part) == nil {
			out = append(out, part...)
		}
	}
	return out
}

// noteWrite keeps the loaded list of this kind in step with a write the
// server accepted: the record at path is replaced by obj, or leaves when obj
// is nil. A count once shown stays until the list is read again (C-6.3); the
// other entries are as that read left them. Nothing loaded, nothing kept.
func (c *collectionCtx) noteWrite(path string, obj *model.Object) {
	// Carrel wrote here: a whole read kept of this collection is not what the
	// server holds now. A server with a change tag would say so on the next
	// read anyway; one without would not until the soft limit, and this
	// session must never show itself a list older than its own write (S-4.2).
	dropWholeReads(c.sess, c.conn.ID, c.col.Path)
	key := c.indexKey(c.spec.component)
	data, ok := c.sess.Cache().PeekETagMap(c.conn.ID, key)
	if !ok {
		return
	}
	var entries []indexEntry
	if json.Unmarshal(data, &entries) != nil {
		c.sess.Cache().InvalidatePath(c.conn.ID, key)
		return
	}
	kept := entries[:0]
	for _, e := range entries {
		if e.Path != path {
			kept = append(kept, e)
		}
	}
	if obj != nil {
		kept = append(kept, indexOf(c.tf, c.spec.kind, c.listed([]*model.Object{obj}))...)
	}
	c.putIndex(kept)
}

// rememberBody keeps the version an edit form was read at, so a rejected
// write can tell what the person changed from what the server changed (S-8).
func (s *Server) rememberBody(c *collectionCtx, obj *model.Object) {
	if body, err := c.body(obj); err == nil {
		c.sess.Cache().PutBody(c.conn.ID, obj.Path, obj.ETag, body)
	}
}

func (s *Server) readVersion(c *collectionCtx, path, etag string) (*model.Object, bool) {
	body, ok := c.sess.Cache().GetBody(c.conn.ID, path, etag)
	if !ok {
		return nil, false
	}
	obj, err := c.parse(path, etag, body)
	return obj, err == nil
}

// unavailableNotice is a source failure on the screen (C-9.1, S-21): what
// failed, the server's answer, and a retry of this source only.
func (s *Server) unavailableNotice(c *collectionCtx, retry string, err error) view.Notice {
	return view.Notice{
		Role:  view.NoticeAlert,
		Title: text(collectionLabel(c.col) + " is unavailable"),
		Body:  text("The server did not answer as expected, so nothing on this screen comes from it."),
		Details: &view.Diagnostic{
			Summary: text("REPORT " + c.col.Path),
			Lines:   []view.Text{text(connLabel(c.conn)), text(err.Error())},
		},
		Actions: []view.Action{{ID: "source.retry", Label: "Try again", Href: s.route(retry), Method: "GET", Enabled: true}},
	}
}

// unreadable is a record that could not be read, with the stamp of its
// collection when the list is combined.
type unreadable struct {
	cal.Unreadable
	stamp view.Stamp
}

func unreadableOf(bad []cal.Unreadable) []unreadable {
	out := make([]unreadable, len(bad))
	for i, b := range bad {
		out[i] = unreadable{Unreadable: b}
	}
	return out
}

func appendUnreadable(notices []view.Notice, bad []unreadable) []view.Notice {
	if len(bad) == 0 {
		return notices
	}
	n := view.Notice{
		Role:    view.NoticeAlert,
		Title:   text(plural(len(bad), "record") + " could not be read"),
		Body:    text("The server returned them, but they are not valid iCalendar. They are left as they are; other clients may still open them."),
		Details: &view.Diagnostic{Summary: text("Unreadable records")},
	}
	for _, b := range bad {
		line := b.Path + " · " + b.Err.Error()
		if b.stamp != "" {
			line = string(b.stamp) + " · " + line
		}
		n.Details.Lines = append(n.Details.Lines, text(line))
	}
	return append(notices, n)
}

// writeFailure is a refused write: nothing saved, the form keeps its values.
// A 403 says what it means (C-9.1).
func (s *Server) writeFailure(c *collectionCtx, method, path string, err error) view.Notice {
	n := view.Notice{
		Role:  view.NoticeAlert,
		Title: text("The server refused the change"),
		Body:  text("Nothing was saved. The form keeps what you entered."),
		Details: &view.Diagnostic{
			Summary: text(method + " " + path),
			Lines:   []view.Text{text(connLabel(c.conn) + path), text(err.Error())},
		},
	}
	if dav.IsStatus(err, http.StatusForbidden) {
		n.Title = text("This account can read here but not write")
	}
	return n
}

func (s *Server) newRecordAction(c *collectionCtx) view.Action {
	a := view.Action{
		ID: "record.new", Label: text("New " + c.spec.noun), Icon: "i-plus",
		Href: s.route(c.listPath() + "/new"), Method: "GET", Primary: true, Enabled: !c.col.ReadOnly,
	}
	if c.col.ReadOnly {
		a.Reason = "This collection is read-only."
	}
	return a
}

// listActions are the operations of one collection's list (S-19): a new
// record, the wizard that brings a file in, and the file it hands over. A
// read-only collection is not offered the wizard — nothing here would write
// it — and is handed over like any other: reading is what it allows. The
// collection is published from its menu in Connections, not from here
// (owner's decision, to-typecase task 12; s-contacts-1, s-agenda-1).
func (s *Server) listActions(c *collectionCtx, export url.Values) []view.Action {
	out := []view.Action{s.newRecordAction(c)}
	if !c.col.ReadOnly {
		out = append(out, s.importAction(c))
	}
	return append(out, s.exportAction(c, export))
}

// listRefresh is the top bar's refresh of one collection's list
// (s-agenda-1; the client's scan of the calendar, А2): the circle reads the
// list again — its address as it stands —, and the words say when this
// session last read the collection.
func (s *Server) listRefresh(c *collectionCtx, r *http.Request) view.Refresh {
	last, _ := readingOf(c.sess, c.tf, c.conn.ID, c.col.Path)
	return view.Refresh{
		Action:   view.Action{ID: "list.refresh", Label: "Refresh", Icon: "i-refresh", Href: s.route(retryPath(c, r)), Method: "GET", Enabled: true},
		Answered: last,
	}
}

func retryPath(c *collectionCtx, r *http.Request) string {
	if r.URL.RawQuery != "" {
		return c.listPath() + "?" + r.URL.RawQuery
	}
	return c.listPath()
}

func (s *Server) getKindList(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, false)
	if !ok {
		return
	}
	if c.spec.kind == view.KindContact || c.spec.kind == view.KindEvent {
		// «Compact rows» of the contacts and of the agenda (S-9.6).
		if s.compactChoice(w, r, c.spec.segment, c.listPath()) {
			return
		}
	}
	if c.spec.kind == view.KindContact || c.spec.kind == view.KindEvent {
		r = withoutOpen(r)
	}
	// The rail is built after the list: the list is what fills its counts.
	// The list leaves the tags of what it read in the shell's rail.
	rail := func(tags []view.Tag) view.Rail {
		rl := s.kindRail(c.sess, s.catalogOf(c.sess), c.spec, nil, c.listPath())
		rl.Tags = tags
		return rl
	}
	// A calendar the server made without some of what was asked says so on
	// its lists (docs/servers.md, SV-52); a list put back in its default
	// order because its column went says that (note 25, s-tasks-13).
	sorted := unsortedNotice(r, c.spec.segment)
	notices := func(own []view.Notice) []view.Notice {
		if sorted != nil {
			own = append(own, *sorted)
		}
		if n := s.droppedNotice(c.sess, c.conn.ID, c.col); n != nil {
			return append([]view.Notice{*n}, own...)
		}
		return own
	}
	var page any
	switch c.spec.kind {
	case view.KindContact:
		p := s.contactsList(r, c)
		p.Rail = rail(p.Rail.Tags)
		p.Notices = notices(p.Notices)
		p.Top.Refresh = s.listRefresh(c, r)
		page = p
	case view.KindEvent:
		p := s.agenda(r, c)
		p.Rail = rail(p.Rail.Tags)
		p.Notices = notices(p.Notices)
		p.Top.Refresh = s.listRefresh(c, r)
		page = p
	case view.KindTask:
		p := s.tasksList(r, c)
		p.Rail = rail(p.Rail.Tags)
		p.Notices = notices(p.Notices)
		p.Top.Refresh = s.listRefresh(c, r)
		page = p
	default:
		p := s.notesList(r, c)
		p.Rail = rail(p.Rail.Tags)
		p.Notices = notices(p.Notices)
		p.Top.Refresh = s.listRefresh(c, r)
		page = p
	}
	s.writePage(w, r, http.StatusOK, page)
}

// listSort is the order a list is shown in (review Р-12, s-contacts-1,
// s-tasks-1, s-notes-1, s-files-1): the server sorts and groups, the client
// neither sorts nor reckons (C-3.1). The first of each kind is its default.
// column is the column of the Columns menu the order sorts by: choosing the
// order shows it, and taking it away puts the list back in its default
// order (note 25, s-tasks-13).
type listSort struct{ id, label, value, column string }

var listSorts = map[view.Kind][]listSort{
	view.KindContact: {{"sort.az", "A–Z", "az", "name"}, {"sort.changed", "Recently changed", "changed", "changed"}},
	view.KindNote: {{"sort.newest", "Newest", "newest", "date"}, {"sort.oldest", "Oldest", "oldest", "date"},
		{"sort.title", "Title", "title", "title"}},
	view.KindTask: {{"sort.due", "By due", "due", "due"}, {"sort.priority", "By priority", "priority", "priority"},
		{"sort.changed", "Recently changed", "changed", "changed"}},
	view.KindFile: {{"files.sort.name", "Name", "name", "name"}, {"files.sort.size", "Size", "size", "size"},
		{"files.sort.changed", "Changed", "changed", "changed"}},
}

// sortOf is the order the address asks for, or the kind's default.
func sortOf(r *http.Request, k view.Kind) string {
	want := strings.TrimSpace(r.FormValue("sort"))
	for _, o := range listSorts[k] {
		if o.value == want {
			return want
		}
	}
	return listSorts[k][0].value
}

// sortSegments are the orders of the kind, the current one on. They keep
// the category filter and the task segment of the address.
func (s *Server) sortSegments(base string, k view.Kind, current string, keep url.Values) []view.Action {
	var out []view.Action
	for _, o := range listSorts[k] {
		q := keptQuery(keep)
		q.Set("sort", o.value)
		f := tagFilter{base: base, query: q, route: s.route}
		out = append(out, view.Action{ID: o.id, Label: text(o.label), Href: f.href(keep.Get("tag")),
			Method: "GET", Enabled: true, On: o.value == current})
	}
	return out
}

// changedAt is when the server last said the record changed, for the
// "Recently changed" order; zero when it never said.
func changedAt(obj *model.Object, tz *time.Location) time.Time {
	if t, err := time.Parse("20060102T150405Z", strings.TrimSpace(firstValue(obj, "REV"))); err == nil {
		return t
	}
	for _, prop := range []string{"LAST-MODIFIED", "DTSTAMP"} {
		if t, ok := cal.TimeOf(obj, prop, tz); ok {
			return t
		}
	}
	return time.Time{}
}

func (s *Server) contactsList(r *http.Request, c *collectionCtx) view.ContactsListPage {
	page := view.ContactsListPage{Shell: s.buildShell(c.sess, collectionLabel(c.col)), ReadOnly: c.col.ReadOnly}
	page.Bar.Actions = append(s.listActions(c, nil), s.printAction(c.listPath()))
	page.Bar.Columns = s.columnsMenu(r, csrfField(c.sess), c.spec.segment, c.listPath()+"/columns", r.URL.Query(), true)
	sift := strings.TrimSpace(r.URL.Query().Get("filter"))
	page.Bar.Filter, page.Bar.FilterForm = text(sift), s.listFilter(c.listPath(), r.URL.Query(), sift)
	page.Bar.Compact = s.compactForm(r, c.spec.segment, c.listPath())
	filter := s.tagFilter(c.listPath(), r.URL.Query())
	objs, err := c.contactsAll(r.Context())
	if err != nil {
		page.Subtitle = text(bookSubtitle(c, 0, false))
		page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
		return page
	}
	page.Subtitle = text(bookSubtitle(c, len(objs), true))
	items := c.listed(objs)
	shown := filterContacts(s.foldLinked(s.catalogOf(c.sess), view.KindContact, filter.keep(view.KindContact, items)), sift)
	by := sortOf(r, view.KindContact)
	page.Groups = s.contactGroups(c.sess, c.tf, shown, c.spec.segment, false, by, filter)
	countGroups(page.Groups)
	page.Details = s.openInPanel(r, c.sess, c.listPath(), shown, page.Groups)
	if len(objs) == 0 {
		page.Empty = bookEmpty(c, page.Bar.Actions)
	}
	page.Bar.Segments = s.sortSegments(c.listPath(), view.KindContact, by, r.URL.Query())
	page.Rail.Tags = filter.rail(view.KindContact, items)
	c.putIndex(indexOf(c.tf, view.KindContact, items))
	c.noteRead(items, true)
	total := len(objs)
	page.Bar.Loaded, page.Bar.Total = len(shown), &total
	page.Print = view.Print{Title: page.Title, Range: printRange(plural(len(shown), "card"), filter.tag), Taken: c.tf.taken(c.tf.now)}
	return page
}

// contactGroups sorts contacts by name under letter rubrics; "#" gathers
// everything that does not start with a letter, last. mark puts the origin on
// every row. A row's avatar is its photo's thumbnail when there is one to show.
// A row carries every column the Columns menu offers (s-contacts-23, -24):
// its tags lead to the list filtered by them.
func (s *Server) contactGroups(sess *session.Session, tf timeFmt, items []listed, segment string, mark bool, by string, filter tagFilter) []view.ContactGroup {
	avatars := s.listAvatars(sess, items)
	names := make([]string, len(items))
	changed := make([]time.Time, len(items))
	order := make([]int, len(items))
	for i, it := range items {
		names[i], order[i] = contactName(it.pick("FN")), i
		changed[i] = changedAt(it.obj, tf.tz)
	}
	sort.SliceStable(order, func(a, b int) bool {
		i, j := order[a], order[b]
		if by == "changed" {
			if !changed[i].Equal(changed[j]) {
				return changed[i].After(changed[j])
			}
			return strings.ToLower(names[i]) < strings.ToLower(names[j])
		}
		ri, rj := rubricOf(names[i]), rubricOf(names[j])
		if ri != rj {
			return rj == "#" || (ri != "#" && ri < rj)
		}
		if x, y := strings.ToLower(names[i]), strings.ToLower(names[j]); x != y {
			return x < y
		}
		if x, y := card.UID(items[i].obj), card.UID(items[j].obj); x != y {
			return x < y
		}
		return items[i].from.listPath(segment) < items[j].from.listPath(segment)
	})
	var groups []view.ContactGroup
	for _, i := range order {
		it, name := items[i], names[i]
		uid := card.UID(it.obj)
		row := view.ContactRow{
			Href:   s.route(it.from.recordPath(segment, uid)),
			Target: render.TargetDetails, // the row opens in the panel (C-7.2)
			UID:    uid,
			Avatar: avatarFor(uid, name),
			Name:   text(name),
			Phone:  text(firstValue(it.obj, "TEL")),
			Email:  text(firstValue(it.obj, "EMAIL")),
			Org:    text(structured(firstValue(it.pick("ORG"), "ORG"))),
			Tags:   filter.tags(categoriesOf(view.KindContact, it.obj)),
			State:  linkedState(it),
		}
		if bday := it.pick("BDAY").Props()["BDAY"]; len(bday) > 0 {
			row.Birthday = text(birthdayTyped(bday[0].Text, bday[0].Params))
		}
		if day := changed[i]; !day.IsZero() {
			row.Changed = text(tf.shortDay(day))
		}
		// A linked row shows the fields of the whole group: what the row
		// lacks comes from the others (S-16).
		for _, o := range it.also {
			if row.Phone == "" {
				row.Phone = text(firstValue(o.obj, "TEL"))
			}
			if row.Email == "" {
				row.Email = text(firstValue(o.obj, "EMAIL"))
			}
			if row.Org == "" {
				row.Org = text(structured(firstValue(o.obj, "ORG")))
			}
			if bday := o.obj.Props()["BDAY"]; row.Birthday == "" && len(bday) > 0 {
				row.Birthday = text(birthdayTyped(bday[0].Text, bday[0].Params))
			}
		}
		row.Badges, row.Sources = linkedBadge(it), linkedSources(it)
		row.Menu = s.recordOps(it.from, recordKinds[segment], uid, it.obj.ETag).rowMenu()
		row.Avatar.Href = avatars[i]
		if mark {
			row.Stamp, row.Colour = it.from.stamp(), catalogColour(it.from.col)
		}
		rubric := rubricOf(name)
		if by == "changed" {
			// One list: the letters say nothing about when a card changed.
			rubric = ""
		}
		if n := len(groups); n == 0 || string(groups[n-1].Rubric) != rubric {
			groups = append(groups, view.ContactGroup{Rubric: text(rubric)})
		}
		g := &groups[len(groups)-1]
		g.Rows = append(g.Rows, row)
	}
	return groups
}

// weekWindow is the agenda week in the instance zone: the week of today, or
// the one starting at from.
func weekWindow(tf timeFmt, from string) (time.Time, time.Time) {
	start := tf.weekStart(tf.now.In(tf.tz))
	if from != "" {
		if d, err := time.ParseInLocation("2006-01-02", from, tf.tz); err == nil {
			start = d
		}
	}
	return start, start.AddDate(0, 0, 7)
}

// The windows of the agenda (review Р-12, s-agenda-1). It stays a list in
// every one of them: Day, Week and Month are how much of it is shown, not a
// grid (U-6).
const (
	agendaDay   = "day"
	agendaWeek  = "week"
	agendaMonth = "month"
	agendaRange = "range"
)

// agendaWindow is the window the address asks for: its kind, its first day
// and the day after its last. An unreadable date, or a range that ends
// before it starts, is the week — the address is never half-obeyed.
func agendaWindow(tf timeFmt, q url.Values) (kind string, from, to time.Time) {
	day := func(name string) (time.Time, bool) {
		d, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(q.Get(name)), tf.tz)
		return d, err == nil
	}
	today := dayOf(tf.now, tf.tz)
	switch q.Get("view") {
	case agendaDay:
		from = today
		if d, ok := day("from"); ok {
			from = d
		}
		return agendaDay, from, from.AddDate(0, 0, 1)
	case agendaMonth:
		from = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, tf.tz)
		if d, ok := day("from"); ok {
			from = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, tf.tz)
		}
		return agendaMonth, from, from.AddDate(0, 1, 0)
	case agendaRange:
		f, okFrom := day("from")
		t, okTo := day("to")
		if okFrom && okTo && !t.Before(f) {
			return agendaRange, f, t.AddDate(0, 0, 1)
		}
	}
	from, to = weekWindow(tf, q.Get("from"))
	return agendaWeek, from, to
}

// untitledOf names a record of the agenda that has no title of its own.
func untitledOf(k view.Kind) string {
	switch k {
	case view.KindTask:
		return "Untitled task"
	case view.KindNote:
		return "Untitled note"
	}
	return "Untitled event"
}

// keptQuery is the address's parameters a segment or a window keeps: not
// the window itself, not the category, and not the poll — another list is
// another poll (S-15.1).
func keptQuery(keep url.Values, drop ...string) url.Values {
	out := url.Values{}
	for name, v := range keep {
		switch name {
		case "tag", "task", "again":
			continue
		}
		if slices.Contains(drop, name) {
			continue
		}
		out[name] = v
	}
	return out
}

// agendaWith are the kinds the agenda shows on its line of days (s-agenda-1,
// U-6: switches, not a choice of one): the values of with — events, tasks
// by their due date, notes by their date. An address without with shows the
// events; the form of the bar (agendaShow) always sends an empty with, so
// a form with nothing ticked is obeyed, not taken for the plain address.
func agendaWith(q url.Values) map[view.Kind]bool {
	vals, spoke := q["with"]
	if !spoke {
		return map[view.Kind]bool{view.KindEvent: true}
	}
	on := map[view.Kind]bool{}
	for _, v := range vals {
		switch strings.TrimSpace(v) {
		case "events":
			on[view.KindEvent] = true
		case "tasks":
			on[view.KindTask] = true
		case "notes":
			on[view.KindNote] = true
		}
	}
	return on
}

// agendaKinds are the ticks of the bar in their order: the value of with,
// the word and the kind.
var agendaKinds = []struct {
	value, label string
	kind         view.Kind
}{{"events", "Events", view.KindEvent}, {"tasks", "Tasks due", view.KindTask}, {"notes", "Notes", view.KindNote}}

// withValues is what an address says of the ticks: nothing for the events
// alone, else every ticked kind — and the empty with, when none is.
func withValues(on map[view.Kind]bool) []string {
	if len(on) == 1 && on[view.KindEvent] {
		return nil
	}
	var out []string
	for _, k := range agendaKinds {
		if on[k.kind] {
			out = append(out, k.value)
		}
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// agendaShow is «Events · Tasks due · Notes» of the bar (s-agenda-1): one
// GET form on the agenda — the window and the category of the address kept
// hidden, an empty with that says the form spoke, and the check with whose
// values are the kinds, each Selected when it is on the line. The poll is
// not kept: other ticks are another poll (S-15.1).
func (s *Server) agendaShow(base string, on map[view.Kind]bool, q url.Values) *view.Form {
	kept := keptQuery(q, "with", "compact")
	if tag := strings.TrimSpace(q.Get("tag")); tag != "" {
		kept.Set("tag", tag)
	}
	var rows []view.FormRow
	for _, name := range slices.Sorted(maps.Keys(kept)) {
		for _, v := range kept[name] {
			rows = append(rows, view.FormRow{Name: name, Kind: view.InputHidden, Value: text(v)})
		}
	}
	check := view.FormRow{Name: "with", Label: "Show", Kind: view.InputCheck}
	for _, k := range agendaKinds {
		check.Options = append(check.Options, view.Option{Value: k.value, Label: text(k.label), Selected: on[k.kind]})
	}
	rows = append(rows, view.FormRow{Name: "with", Kind: view.InputHidden, Value: ""}, check)
	f := s.plainForm(base, "GET", rows...)
	return &f
}

// todayQuery is the window of today: the week is the address without a view
// at all, so the plainest address stays the plainest.
func todayQuery(kind string) url.Values {
	if kind == agendaWeek {
		return url.Values{}
	}
	return url.Values{"view": {kind}}
}

// agendaRange is the window on the screen: Day, Week and Month to switch
// between, the one before and the one after, and a range of one's own. It
// keeps the category filter and the segment of the address.
func (s *Server) agendaRange(tf timeFmt, base, tag, kind string, from, to time.Time, keep url.Values) *view.Range {
	href := func(set url.Values) string {
		q := keptQuery(keep, "view", "from", "to")
		for name, v := range set {
			q[name] = v
		}
		return tagFilter{base: base, query: q, route: s.route}.href(tag)
	}
	window := func(k string, day time.Time) string {
		set := url.Values{"view": {k}, "from": {day.Format("2006-01-02")}}
		if k == agendaWeek {
			delete(set, "view") // the week is the address without a view
		}
		return href(set)
	}
	back, ahead := from, to
	switch kind {
	case agendaDay:
		back, ahead = from.AddDate(0, 0, -1), to
	case agendaMonth:
		back, ahead = from.AddDate(0, -1, 0), to
	case agendaRange:
		span := to.Sub(from)
		back, ahead = from.Add(-span), to
	default:
		back, ahead = from.AddDate(0, 0, -7), to
	}
	rng := &view.Range{From: text(tf.date(from)), To: text(tf.date(to.AddDate(0, 0, -1)))}
	if kind != agendaRange {
		// Previous · Today · Next lead the bar (s-agenda-1, task 77): the
		// steps are icons named by the step — «Previous week».
		step := map[string]string{agendaDay: "day", agendaMonth: "month"}[kind]
		if step == "" {
			step = "week"
		}
		rng.Presets = []view.Action{
			{ID: "range.previous", Label: text("Previous " + step), Icon: "i-back", Href: window(kind, back), Method: "GET", Enabled: true},
			{ID: "range.current", Label: "Today", Href: href(todayQuery(kind)), Method: "GET", Enabled: true},
			{ID: "range.next", Label: text("Next " + step), Icon: "i-next", Href: window(kind, ahead), Method: "GET", Enabled: true},
		}
	}
	// Switching the window keeps where the person is: today when today is in
	// it, else its first day.
	anchor := from
	if today := dayOf(tf.now, tf.tz); !today.Before(from) && today.Before(to) {
		anchor = today
	}
	for _, w := range []struct{ id, label, kind string }{
		{"range.day", "Day", agendaDay}, {"range.week", "Week", agendaWeek}, {"range.month", "Month", agendaMonth},
	} {
		rng.Presets = append(rng.Presets, view.Action{ID: w.id, Label: text(w.label), Method: "GET", Enabled: true,
			On: w.kind == kind, Href: window(w.kind, anchor)})
	}
	// Range — the window on the screen as a range of its own, its days in
	// the form below (s-agenda-1).
	rng.Presets = append(rng.Presets, view.Action{ID: "range.range", Label: "Range", Method: "GET", Enabled: true, On: kind == agendaRange,
		Href: href(url.Values{"view": {agendaRange}, "from": {from.Format("2006-01-02")}, "to": {to.AddDate(0, 0, -1).Format("2006-01-02")}})})
	// A GET form puts its fields in the address: no _csrf here (C-11, the
	// client's scan Б6). To has no hint: the frame draws none (s-agenda-1).
	form := s.plainForm(base, "GET",
		view.FormRow{Name: "view", Kind: view.InputHidden, Value: agendaRange},
		view.FormRow{Name: "from", Label: "From", Kind: view.InputDate, Value: text(from.Format("2006-01-02")), Mono: true},
		view.FormRow{Name: "to", Label: "To", Kind: view.InputDate, Value: text(to.AddDate(0, 0, -1).Format("2006-01-02")), Mono: true},
	)
	form.Actions = []view.Action{{ID: "range.show", Label: "Show", Href: s.route(base), Method: "GET", Enabled: true}}
	rng.Form = &form
	return rng
}

// openInAgenda answers ?open={a row's href} on an agenda without scripts
// (s-agenda-1): every row of that record is on, and the record stands in the
// panel, read; Close is the agenda without it. With scripts the panel is a
// fragment at the record's address (C-7.2).
func (s *Server) openInAgenda(r *http.Request, sess *session.Session, base string, items []kinded, days []view.AgendaDay) *view.Details {
	open, _ := r.Context().Value(openKey{}).(string)
	if open == "" {
		return nil
	}
	for _, it := range items {
		uid := cal.UID(it.obj)
		segment := segmentOf(it.kind)
		if s.route(it.from.recordPath(segment, uid)) != open {
			continue
		}
		c, ok := s.collectionOf(sess, recordKinds[segment], it.from.conn.ID, it.from.col.Path)
		if !ok {
			return nil
		}
		for di := range days {
			for ri := range days[di].Rows {
				if days[di].Rows[ri].Href == open {
					days[di].Rows[ri].On = true
				}
			}
		}
		d := s.recordDetails(c, uid, it.obj)
		d.Close = s.route(base)
		if r.URL.RawQuery != "" {
			d.Close += "?" + r.URL.RawQuery
		}
		return &d
	}
	return nil
}

// agendaSubtitle is the line under the title of the combined calendar
// (s-agenda-1): «5 of 7 calendars · 3 accounts», the days of the window and
// the instance zone, monospaced.
func agendaSubtitle(tf timeFmt, where string, from, to time.Time) view.Runs {
	return view.Runs{{Text: text(where + " · " + tf.window(from, to) + " · ")}, {Text: text(tf.tz.String()), Mark: view.RunMono}}
}

// calendarSubtitle is the line under one calendar's title (states-15):
// its connection, the days of the window and how many events are in it —
// «nextcloud · 2 – 8 November 2026 · 0 events»; without the count when the
// calendar did not answer.
func calendarSubtitle(tf timeFmt, where string, from, to time.Time, events *int) view.Runs {
	line := where + " · " + tf.window(from, to)
	if events != nil {
		line += " · " + plural(*events, "event")
	}
	return view.Runs{{Text: text(line)}}
}

// eventsIn counts the events of an agenda's days: the tasks due and the
// notes the bar may put among them are not events.
func eventsIn(days []view.AgendaDay) int {
	n := 0
	for _, d := range days {
		for _, r := range d.Rows {
			if r.Kind == view.KindEvent {
				n++
			}
		}
	}
	return n
}

// windowEmpty is what an agenda says when its window holds nothing (the
// owner's decision of 02.10.2026, states-15): one line for the window, not
// a rubric for each of its days — «No events this week», «Nothing in
// Личный from 2 to 8 November.»; where is the calendar, or the ticked ones.
func windowEmpty(tf timeFmt, kind, where string, from, to time.Time) *view.Notice {
	title := map[string]string{agendaDay: "No events this day", agendaMonth: "No events this month",
		agendaRange: "No events in this range"}[kind]
	if title == "" {
		title = "No events this week"
	}
	a, b := from.In(tf.tz), to.In(tf.tz).AddDate(0, 0, -1)
	var when string
	switch {
	case tf.dayKey(a) == tf.dayKey(b) || b.Before(a):
		when = "on " + a.Format("2 January")
	case a.Year() != b.Year():
		when = "from " + a.Format("2 January 2006") + " to " + b.Format("2 January 2006")
	case a.Month() != b.Month():
		when = "from " + a.Format("2 January") + " to " + b.Format("2 January")
	default:
		when = "from " + a.Format("2") + " to " + b.Format("2 January")
	}
	return &view.Notice{Role: view.NoticeInfo, Title: text(title), Body: text("Nothing in " + where + " " + when + "."), Actions: []view.Action{}}
}

// agendaPrintAction is Print of an agenda's head (U-10): the same address,
// its window and ticks, with print=1.
func (s *Server) agendaPrintAction(base string, q url.Values) view.Action {
	a := s.printAction(base)
	q = keptQuery(q, render.PrintParam, "compact")
	q.Set(render.PrintParam, render.PrintValue)
	a.Href = s.route(base) + "?" + q.Encode()
	return a
}

// agendaPrint is the week of an agenda in its print header.
func agendaPrint(tf timeFmt, from, to time.Time) string {
	return tf.date(from) + " – " + tf.date(to.AddDate(0, 0, -1))
}

// agenda is one week of one calendar in the instance zone. Without a window
// the calendar is never read whole (S-11); the window goes to the server as
// the REPORT time-range.
func (s *Server) agenda(r *http.Request, c *collectionCtx) view.AgendaPage {
	tf := c.tf
	kind, from, to := agendaWindow(tf, r.URL.Query())
	page := view.AgendaPage{Shell: s.buildShell(c.sess, collectionLabel(c.col)), Days: []view.AgendaDay{}}
	page.Subtitle = calendarSubtitle(tf, connLabel(c.conn), from, to, nil)
	// The export of a calendar takes the week the screen shows: that is where
	// the range is (S-19). Every other list is handed over whole.
	page.Bar.Actions = append(s.listActions(c, url.Values{
		"from": {from.Format("2006-01-02")}, "to": {to.AddDate(0, 0, -1).Format("2006-01-02")},
	}), s.agendaPrintAction(c.listPath(), r.URL.Query()))
	filter := s.tagFilter(c.listPath(), r.URL.Query())
	page.Bar.Range = s.agendaRange(tf, c.listPath(), filter.tag, kind, from, to, r.URL.Query())
	on := agendaWith(r.URL.Query())
	page.Bar.Show, page.Bar.Segments = s.agendaShow(c.listPath(), on, r.URL.Query()), []view.Action{}
	page.Bar.Compact = s.compactForm(r, c.spec.segment, c.listPath())

	objs, bad, err := c.calQuery(r.Context(), c.spec.component, &from, &to)
	if err != nil {
		page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
		return page
	}
	items := c.listed(objs)
	var shown []kinded
	if on[view.KindEvent] {
		shown = kindedAll(s.foldLinked(s.catalogOf(c.sess), view.KindEvent, filter.keep(view.KindEvent, items)), view.KindEvent)
	}
	// Tasks due and notes come from the same collection, each by its own
	// component (s-agenda-1); a collection that holds none is simply silent.
	for _, k := range []view.Kind{view.KindTask, view.KindNote} {
		if !on[k] || !c.col.Has(k) {
			continue
		}
		// Whole, not by time-range: a VTODO is placed by DUE and a VJOURNAL
		// by DTSTART, and servers filter neither reliably (docs/servers.md).
		// The window is applied here, where the date is read anyway.
		more, moreBad, err := c.calQuery(r.Context(), recordKinds[segmentOf(k)].component, nil, nil)
		if err != nil {
			page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
			continue
		}
		bad = append(bad, moreBad...)
		shown = append(shown, kindedAll(filter.keep(k, c.listed(more)), k)...)
	}
	var more []unreadable
	page.Days, page.Bar.Loaded, more = s.agendaDays(tf, shown, from, to, c.spec.segment, false)
	events := eventsIn(page.Days)
	page.Subtitle = calendarSubtitle(tf, connLabel(c.conn), from, to, &events)
	if len(page.Days) == 0 && len(page.Notices) == 0 {
		page.Empty = windowEmpty(tf, kind, collectionLabel(c.col), from, to)
	}
	page.Details = s.openInAgenda(r, c.sess, c.listPath(), shown, page.Days)
	page.Notices = appendUnreadable(page.Notices, append(unreadableOf(bad), more...))
	page.Rail.Tags = filter.rail(view.KindEvent, items)
	c.putIndex(indexOf(c.tf, view.KindEvent, items))
	c.noteRead(items, false)
	page.Print = view.Print{
		Title: page.Title, Range: printRange(agendaPrint(tf, from, to), plural(page.Bar.Loaded, "event"), filter.tag),
		Taken: tf.taken(tf.now),
	}
	return page
}

// agendaDays expands series into the window and lays an instance another
// client rewrote (RECURRENCE-ID) over the instant it replaces; the rows go
// under day rubrics, all-day first, then by time.
// kinded is a record of the agenda with the kind it is shown as: an event,
// a task by its due date, a note by its date.
type kinded struct {
	listed
	kind view.Kind
}

func kindedAll(items []listed, k view.Kind) []kinded {
	out := make([]kinded, 0, len(items))
	for _, it := range items {
		out = append(out, kinded{listed: it, kind: k})
	}
	return out
}

func (s *Server) agendaDays(tf timeFmt, items []kinded, from, to time.Time, segment string, mark bool) ([]view.AgendaDay, int, []unreadable) {
	tz := tf.tz
	type occurrence struct {
		at     time.Time
		allDay bool
		order  string
		row    view.AgendaRow
	}
	var occs []occurrence
	var bad []unreadable
	for _, it := range items {
		o := it.obj
		uid := cal.UID(o)
		if it.kind != view.KindEvent {
			// A task stands on the day it is due, a note on its date; one
			// row each, no recurrence to expand here (S-11).
			at := timelineWhen(it.kind, o, tz)
			if at.IsZero() || at.Before(from) || !at.Before(to) {
				continue
			}
			row := view.AgendaRow{
				Kind:    it.kind,
				Href:    s.route(it.from.recordPath(segmentOf(it.kind), uid)),
				Target:  render.TargetDetails,
				UID:     uid,
				Time:    text(tf.timeLabel(at, false)),
				Summary: text(titleOr(cal.Text(it.pick("SUMMARY"), "SUMMARY"), untitledOf(it.kind))),
				State:   linkedState(it.listed),
				Menu:    s.recordOps(it.from, recordKinds[segmentOf(it.kind)], uid, o.ETag).rowMenu(),
			}
			if it.kind == view.KindTask {
				row.Badges = append(row.Badges, view.Badge{Label: "Due", Role: view.BadgeNeutral})
			}
			if mark {
				row.Stamp, row.Colour = it.from.stamp(), catalogColour(it.from.col)
			}
			occs = append(occs, occurrence{at: at, order: string(it.from.stamp()) + "\x00" + it.from.listPath(segment), row: row})
			continue
		}
		summary := titleOr(cal.Text(o, "SUMMARY"), "Untitled event")
		tm, err := cal.TimingOf(o, tz)
		var instants []time.Time
		if err == nil {
			instants, err = cal.Expand(o, from, to, tz)
		}
		var overrides []cal.Override
		if err == nil {
			overrides, err = cal.Overrides(o, tz)
		}
		if err != nil {
			u := unreadable{Unreadable: cal.Unreadable{Path: o.Path, UID: uid, Err: err}}
			if mark {
				u.stamp = it.from.stamp()
			}
			bad = append(bad, u)
			continue
		}
		// Every occurrence is the one record: the series is edited and
		// deleted whole.
		menu := s.recordOps(it.from, recordKinds[segment], uid, o.ETag).rowMenu()
		for _, at := range instants {
			if at.Before(from) || !at.Before(to) {
				continue
			}
			occ := occurrence{at: at, allDay: tm.AllDay, order: string(it.from.stamp()) + "\x00" + it.from.listPath(segment), row: view.AgendaRow{
				Kind:     view.KindEvent,
				Href:     s.route(it.from.recordPath(segment, uid)),
				Target:   render.TargetDetails,
				UID:      uid,
				Time:     text(tf.timeLabel(at, tm.AllDay)),
				Summary:  text(titleOr(cal.Text(it.pick("SUMMARY"), "SUMMARY"), summary)),
				Location: text(cal.Text(it.pick("LOCATION"), "LOCATION")),
				State:    linkedState(it.listed),
				Badges:   agendaBadges(o, it.listed),
				Sources:  linkedSources(it.listed),
				Menu:     menu,
			}}
			if mark {
				occ.row.Stamp, occ.row.Colour = it.from.stamp(), catalogColour(it.from.col)
			}
			for _, ov := range overrides {
				if !ov.RecurrenceID.Equal(at) {
					continue
				}
				if !ov.Timing.Start.IsZero() {
					occ.at, occ.allDay = ov.Timing.Start, ov.Timing.AllDay
					occ.row.Time = text(tf.timeLabel(ov.Timing.Start, ov.Timing.AllDay))
				}
				if ov.Summary != "" {
					occ.row.Summary = text(ov.Summary)
				}
				if ov.Location != "" {
					occ.row.Location = text(ov.Location)
				}
				if ov.Cancelled {
					occ.row.Badges = append([]view.Badge{{Label: "Cancelled", Role: view.BadgeAlert}}, occ.row.Badges...)
				}
			}
			occs = append(occs, occ)
		}
	}

	sort.SliceStable(occs, func(i, j int) bool {
		if di, dj := tf.dayKey(occs[i].at), tf.dayKey(occs[j].at); di != dj {
			return di < dj
		}
		if occs[i].allDay != occs[j].allDay {
			return occs[i].allDay
		}
		if !occs[i].at.Equal(occs[j].at) {
			return occs[i].at.Before(occs[j].at)
		}
		if occs[i].row.Summary != occs[j].row.Summary {
			return occs[i].row.Summary < occs[j].row.Summary
		}
		return occs[i].order < occs[j].order
	})
	// A day without events is not printed (the owner's decision of
	// 02.10.2026, task 77): a week with events has no empty rubric
	// (s-agenda-1), and a window without any is one line, the page's Empty
	// (states-15; windowEmpty).
	byDay := map[string][]view.AgendaRow{}
	for _, o := range occs {
		k := tf.dayKey(o.at)
		byDay[k] = append(byDay[k], o.row)
	}
	days := []view.AgendaDay{}
	for day := from; day.Before(to); day = day.AddDate(0, 0, 1) {
		if rows := byDay[tf.dayKey(day)]; len(rows) > 0 {
			days = append(days, view.AgendaDay{Rubric: text(tf.dayRubric(day)), Count: len(rows), Rows: rows})
		}
	}
	return days, len(occs), bad
}

// agendaBadges are the badges of an event's row, the first of them the one
// the row of the frame draws (s-agenda-1): a status that is not plain —
// «tentative» or «cancelled», as an alarm —, the linked group, how many
// people take part, and the repeat.
func agendaBadges(o *model.Object, it listed) []view.Badge {
	var out []view.Badge
	switch strings.ToUpper(strings.TrimSpace(cal.Text(o, "STATUS"))) {
	case "TENTATIVE":
		out = append(out, view.Badge{Label: "tentative", Role: view.BadgeAlert})
	case "CANCELLED":
		out = append(out, view.Badge{Label: "cancelled", Role: view.BadgeAlert})
	}
	out = append(out, linkedBadge(it)...)
	if n := len(eventPeople(o, nil)); n > 0 {
		out = append(out, view.Badge{Label: text(plural(n, "attendee")), Role: view.BadgeNeutral})
	}
	if cal.HasRecurrence(o) {
		out = append(out, view.Badge{Label: text(repeatsWord(cal.RawRRULE(o))), Role: view.BadgeNeutral})
	}
	return out
}

// repeatsWord is the badge of a series in the agenda: «repeats weekly»,
// «repeats every 2 weeks», and «repeats» for a rule the words do not cover
// or extra dates alone.
func repeatsWord(raw string) string {
	r := parseRule(raw)
	words := map[string][2]string{"DAILY": {"daily", "day"}, "WEEKLY": {"weekly", "week"},
		"MONTHLY": {"monthly", "month"}, "YEARLY": {"yearly", "year"}}
	w, ok := words[strings.ToUpper(r.get("FREQ"))]
	if !ok {
		return "repeats"
	}
	if n, err := strconv.Atoi(r.get("INTERVAL")); err == nil && n > 1 {
		return "repeats every " + plural(n, w[1])
	}
	return "repeats " + w[0]
}

// eventPeople are the people of an event as its card lists them
// (s-agenda-1, s-agenda-2): the organiser first, as one of them («4
// attendees» of three ATTENDEE and the ORGANIZER), then the attendees. An
// organiser who is an attendee too is listed once, as the organiser.
func eventPeople(o *model.Object, known map[string]knownPerson) []view.AttendeeRow {
	var out []view.AttendeeRow
	row := func(name, email, status string) view.AttendeeRow {
		p, ok := known[dupes.NormEmail(email)]
		if name == "" {
			name = p.name
		}
		if name == "" {
			name = email
		}
		r := view.AttendeeRow{Name: text(name), Email: text(email), Status: text(status)}
		if ok {
			r.Href, r.Colour = p.href, p.colour
		}
		return r
	}
	organiser := ""
	for _, v := range o.Props()["ORGANIZER"] {
		organiser = cal.CalAddress(v.Text)
		out = append(out, row(v.Params["CN"], organiser, "organiser"))
		break
	}
	for _, a := range cal.Attendees(o) {
		if organiser != "" && strings.EqualFold(a.Email, organiser) {
			continue
		}
		out = append(out, row(a.Name, a.Email, partstatLabel(a.Status)))
	}
	return out
}

// knownPerson is a card an address of an event is found in: who, where it
// is read, and the colour of its book.
type knownPerson struct {
	name, href string
	colour     view.Colour
}

// knownPeople (attendees.go) are the people of the ticked address books
// whose lists the session has read, by their addresses (task 77: an attendee
// is an address, the books say who it is — s-agenda-1, -2). No server is
// asked: as Related, what is not loaded is not known.

func taskShow(r *http.Request) string {
	show := r.URL.Query().Get("show")
	if show != "done" && show != "all" {
		show = "open"
	}
	return show
}

func (s *Server) tasksList(r *http.Request, c *collectionCtx) view.TasksPage {
	show := taskShow(r)
	page := view.TasksPage{Shell: s.buildShell(c.sess, collectionLabel(c.col))}
	page.Bar.Actions = s.listActions(c, nil)
	page.Bar.Columns = s.columnsMenu(r, csrfField(c.sess), c.spec.segment, c.listPath()+"/columns", r.URL.Query(), true)
	filter := s.tagFilter(c.listPath(), r.URL.Query())
	objs, bad, err := c.calQuery(r.Context(), c.spec.component, nil, nil)
	if err != nil {
		page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
		return page
	}
	items := c.listed(objs)
	var open, done int
	var more []unreadable
	by := sortOf(r, view.KindTask)
	page.Groups, open, done, page.Bar.Loaded, more = s.taskGroups(c.tf, show, s.foldLinked(s.catalogOf(c.sess), view.KindTask, filter.keep(view.KindTask, items)), c.spec.segment, false, filter, c.tf.now, by)
	page.Bar.Segments = append(s.taskSegments(c.listPath(), filter.tag, show, open, done),
		s.sortSegments(c.listPath(), view.KindTask, by, r.URL.Query())...)
	page.Notices = appendUnreadable(page.Notices, append(unreadableOf(bad), more...))
	page.Rail.Tags = filter.rail(view.KindTask, items)
	c.putIndex(indexOf(c.tf, view.KindTask, items))
	c.noteRead(items, true)
	page.Print = view.Print{
		Title: page.Title, Range: printRange(showLabels[show], plural(page.Bar.Loaded, "task"), filter.tag),
		Taken: c.tf.taken(c.tf.now),
	}
	return page
}

// taskGroups shows open, done or all tasks: open first, then by due date,
// undated last. A row's categories link to the list filtered to each.
func (s *Server) taskGroups(tf timeFmt, show string, items []listed, segment string, mark bool, filter tagFilter, now time.Time, by string) (groups []view.TaskGroup, open, done, loaded int, bad []unreadable) {
	type item struct {
		row      view.TaskRow
		due      time.Time
		priority int
		changed  time.Time
		order    string
	}
	var rows []item
	for _, it := range items {
		o := it.obj
		uid := cal.UID(o)
		isDone := taskDone(o)
		if isDone {
			done++
		} else {
			open++
		}
		if (show == "open" && isDone) || (show == "done" && !isDone) {
			continue
		}
		due, err := cal.DueOf(o, tf.tz)
		if err != nil {
			u := unreadable{Unreadable: cal.Unreadable{Path: o.Path, UID: uid, Err: err}}
			if mark {
				u.stamp = it.from.stamp()
			}
			bad = append(bad, u)
			continue
		}
		row := view.TaskRow{
			Href: s.route(it.from.recordPath(segment, uid)), Target: render.TargetDetails, UID: uid, Done: isDone,
			Summary: text(titleOr(cal.Text(it.pick("SUMMARY"), "SUMMARY"), "Untitled task")),
			Percent: percentOf(o), Tags: filter.tags(categoriesOf(view.KindTask, o)), State: linkedState(it),
			Badges: linkedBadge(it), Sources: linkedSources(it),
		}
		if mark {
			row.Stamp, row.Colour = it.from.stamp(), catalogColour(it.from.col)
		}
		row.Menu = s.recordOps(it.from, recordKinds[segment], uid, o.ETag).rowMenu()
		if !it.from.col.ReadOnly && !it.from.conn.Feed {
			// The box of the row is its own control, not an item of the
			// menu (components-3).
			from := "list"
			if mark {
				from = "all"
			}
			tick := s.doneAction(it.from.recordPath(segment, uid)+"/edit", o.ETag, isDone, from)
			row.Tick = &tick
		}
		if !due.Start.IsZero() {
			row.Due = text(tf.shortDue(due))
			row.Overdue = !isDone && due.Start.Before(now)
		}
		// The columns the menu adds (s-tasks-12, -13): the priority by its
		// number, the status by the word of its list — a task without one
		// needs action — and the last change with its time.
		if p := priorityOf(o); p <= 9 {
			row.Priority = text(strconv.Itoa(p))
		}
		row.Status = text(statusLabel(firstValue(o, "STATUS")))
		if strings.TrimSpace(firstValue(o, "STATUS")) == "" {
			row.Status = text(taskStatuses[0].label)
		}
		changed := changedAt(o, tf.tz)
		if !changed.IsZero() {
			row.Changed = text(tf.shortStamp(changed))
		}
		rows = append(rows, item{row: row, due: due.Start, priority: priorityOf(o), changed: changed, order: it.from.listPath(segment)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.row.Done != b.row.Done {
			return !a.row.Done
		}
		switch by {
		case "priority":
			// 1 is the highest and 0 is none (RFC 5545 §3.8.1.9): none last.
			if a.priority != b.priority {
				return a.priority < b.priority
			}
		case "changed":
			if !a.changed.Equal(b.changed) {
				return a.changed.After(b.changed)
			}
		default:
			if a.due.IsZero() != b.due.IsZero() {
				return !a.due.IsZero()
			}
			if !a.due.Equal(b.due) {
				return a.due.Before(b.due)
			}
		}
		if a.row.Summary != b.row.Summary {
			return a.row.Summary < b.row.Summary
		}
		return a.order < b.order
	})
	for _, it := range rows {
		rubric := ""
		if by == "due" {
			rubric = dueRubric(it.due, it.row.Done, now)
		}
		if n := len(groups); n == 0 || string(groups[n-1].Rubric) != rubric {
			groups = append(groups, view.TaskGroup{Rubric: text(rubric)})
		}
		g := &groups[len(groups)-1]
		g.Rows = append(g.Rows, it.row)
	}
	return groups, open, done, len(rows), bad
}

// priorityOf is PRIORITY as a number to sort by; 0 — none — sorts last, so
// it answers 10 (RFC 5545 §3.8.1.9: 1 is the highest, 9 the lowest).
func priorityOf(obj *model.Object) int {
	n, err := strconv.Atoi(strings.TrimSpace(firstValue(obj, "PRIORITY")))
	if err != nil || n < 1 || n > 9 {
		return 10
	}
	return n
}

// dueRubric groups tasks by their due date the way the frame does
// (s-tasks-1): what is late, what is due this week, what is dated later,
// and what has no date at all. A task already done is not late.
func dueRubric(due time.Time, done bool, now time.Time) string {
	switch {
	case due.IsZero():
		return "No due date"
	case !done && due.Before(now):
		return "Overdue"
	case due.Before(now.AddDate(0, 0, 7)):
		// The seven days ahead, not the calendar week: on a Sunday evening
		// "this week" would otherwise be empty and tomorrow would be "later".
		return "This week"
	}
	return "Later"
}

// taskSegments switch between open, done and all, and keep the category
// filter: «Open 12», «Done 48», «All 60», as s-tasks-1 writes them — the
// label and its count apart (C-6.2). The one shown now is on and stays a
// choice.
func (s *Server) taskSegments(base, tag, show string, open, done int) []view.Action {
	segment := func(id, label, value string, n int) view.Action {
		f := tagFilter{base: base, query: url.Values{"show": {value}}, route: s.route}
		return view.Action{ID: id, Label: text(label), Tally: text(strconv.Itoa(n)), Href: f.href(tag), Method: "GET", Enabled: true, On: show == value}
	}
	return []view.Action{
		segment("tasks.open", "Open", "open", open),
		segment("tasks.done", "Done", "done", done),
		segment("tasks.all", "All", "all", open+done),
	}
}

func (s *Server) notesList(r *http.Request, c *collectionCtx) view.NotesPage {
	page := view.NotesPage{Shell: s.buildShell(c.sess, collectionLabel(c.col))}
	page.Bar.Actions = s.listActions(c, nil)
	page.Bar.Columns = s.columnsMenu(r, csrfField(c.sess), c.spec.segment, c.listPath()+"/columns", r.URL.Query(), true)
	filter := s.tagFilter(c.listPath(), r.URL.Query())
	objs, bad, err := c.calQuery(r.Context(), c.spec.component, nil, nil)
	if err != nil {
		page.Notices = append(page.Notices, s.unavailableNotice(c, retryPath(c, r), err))
		return page
	}
	items := c.listed(objs)
	var more []unreadable
	by := sortOf(r, view.KindNote)
	page.Groups, page.Bar.Loaded, more = s.noteGroups(c.tf, s.foldLinked(s.catalogOf(c.sess), view.KindNote, filter.keep(view.KindNote, items)), c.spec.segment, false, filter, by)
	page.Bar.Segments = s.sortSegments(c.listPath(), view.KindNote, by, r.URL.Query())
	page.Notices = appendUnreadable(page.Notices, append(unreadableOf(bad), more...))
	page.Rail.Tags = filter.rail(view.KindNote, items)
	c.putIndex(indexOf(c.tf, view.KindNote, items))
	c.noteRead(items, true)
	page.Print = view.Print{Title: page.Title, Range: printRange(plural(page.Bar.Loaded, "note"), filter.tag), Taken: c.tf.taken(c.tf.now)}
	return page
}

// noteGroups lists notes newest first. A row's categories link to the list
// filtered to each.
func (s *Server) noteGroups(tf timeFmt, items []listed, segment string, mark bool, filter tagFilter, by string) ([]view.NoteGroup, int, []unreadable) {
	type item struct {
		row   view.NoteRow
		at    time.Time
		order string
	}
	rows := make([]item, 0, len(items))
	for _, it := range items {
		o := it.obj
		uid, body := cal.UID(o), cal.Description(o)
		title := noteTitle(cal.Text(it.pick("SUMMARY"), "SUMMARY"), body)
		at := noteTime(o, tf.tz)
		row := view.NoteRow{
			Href: s.route(it.from.recordPath(segment, uid)), Target: render.TargetDetails, UID: uid, Title: text(title), Excerpt: text(noteExcerpt(body, title)),
			Tags: filter.tags(categoriesOf(view.KindNote, o)), State: linkedState(it),
			Badges: linkedBadge(it), Sources: linkedSources(it),
			Menu: s.recordOps(it.from, recordKinds[segment], uid, o.ETag).rowMenu(),
		}
		if mark {
			row.Stamp, row.Colour = it.from.stamp(), catalogColour(it.from.col)
		}
		if !at.IsZero() {
			row.Date = text(tf.shortDay(at))
		}
		if changed := changedAt(o, tf.tz); !changed.IsZero() {
			row.Changed = text(tf.shortStamp(changed))
		}
		rows = append(rows, item{row: row, at: at, order: it.from.listPath(segment)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		switch {
		case by == "title":
			if x, y := strings.ToLower(string(rows[i].row.Title)), strings.ToLower(string(rows[j].row.Title)); x != y {
				return x < y
			}
		case !rows[i].at.Equal(rows[j].at):
			if by == "oldest" {
				return rows[i].at.Before(rows[j].at)
			}
			return rows[i].at.After(rows[j].at)
		}
		if rows[i].row.Title != rows[j].row.Title {
			return rows[i].row.Title < rows[j].row.Title
		}
		return rows[i].order < rows[j].order
	})
	var groups []view.NoteGroup
	if len(rows) > 0 {
		group := view.NoteGroup{}
		for _, it := range rows {
			group.Rows = append(group.Rows, it.row)
		}
		groups = []view.NoteGroup{group}
	}
	return groups, len(rows), nil
}

func firstValue(obj *model.Object, prop string) string {
	if v := obj.PropText(prop); len(v) > 0 {
		return v[0]
	}
	return ""
}

// structured shows a vCard structured value (ADR, ORG, N) as its non-empty
// parts.
func structured(v string) string {
	var parts []string
	for _, p := range strings.Split(v, ";") {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, ", ")
}

func contactName(obj *model.Object) string {
	if v := strings.TrimSpace(firstValue(obj, "FN")); v != "" {
		return v
	}
	if n := strings.Split(firstValue(obj, "N"), ";"); len(n) > 1 {
		if v := strings.TrimSpace(strings.TrimSpace(n[1]) + " " + strings.TrimSpace(n[0])); v != "" {
			return v
		}
	}
	if v := firstValue(obj, "EMAIL"); v != "" {
		return v
	}
	return card.UID(obj)
}

func rubricOf(name string) string {
	r, _ := utf8.DecodeRuneInString(strings.TrimSpace(name))
	if unicode.IsLetter(r) {
		return string(unicode.ToUpper(r))
	}
	return "#"
}

func initials(name string) string {
	words := strings.Fields(name)
	var out []rune
	for i, w := range words {
		if i != 0 && i != len(words)-1 {
			continue
		}
		if r, _ := utf8.DecodeRuneInString(w); unicode.IsLetter(r) {
			out = append(out, unicode.ToUpper(r))
		}
	}
	return string(out)
}

// avatarFor is the placeholder: initials on a colour derived from the UID.
func avatarFor(uid, name string) view.Avatar {
	return view.Avatar{Initials: text(initials(name)), Colour: collectionColour(uid)}
}

func titleOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func taskDone(obj *model.Object) bool {
	return strings.EqualFold(cal.Text(obj, "STATUS"), "COMPLETED") || len(obj.PropText("COMPLETED")) > 0
}

func percentOf(obj *model.Object) *int {
	n, err := strconv.Atoi(strings.TrimSpace(firstValue(obj, "PERCENT-COMPLETE")))
	if err != nil {
		return nil
	}
	return &n
}

// noteTitle is SUMMARY, else the first line of the body without its heading
// marks.
func noteTitle(summary, body string) string {
	if strings.TrimSpace(summary) != "" {
		return summary
	}
	for _, line := range strings.Split(body, "\n") {
		if line = strings.TrimSpace(strings.TrimLeft(line, "# ")); line != "" {
			return line
		}
	}
	return "Untitled note"
}

// noteExcerpt is the first line of the body that is not the title. The
// widget shortens it (C-10); the server does not.
func noteExcerpt(body, title string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(line, "# "))
		if line != "" && line != title {
			return line
		}
	}
	return ""
}

func noteTime(obj *model.Object, tz *time.Location) time.Time {
	for _, prop := range []string{"DTSTART", "LAST-MODIFIED", "DTSTAMP"} {
		if t, ok := cal.TimeOf(obj, prop, tz); ok {
			return t
		}
	}
	return time.Time{}
}

// landedOnTheFirstTry reports whether a refusal means the write Carrel sent
// had already arrived. The guard re-sends a write whose answer a closed
// connection ate (SV-9), and a server that already holds the thing refuses
// the second copy: 405 to MKCOL of a collection that exists, 412 to a PUT
// with If-None-Match (S-8). Both readings were «the server would not make
// it», and the full live run of 21.09.2026 caught the lie twice on one
// Nextcloud — a record and an address book were made and the person was
// told they were not.
//
// Only the guard can set the mark, and what follows a true answer here
// reads the address back, so this can shorten a wrong guess into a check,
// never into a record that is not there.
func landedOnTheFirstTry(err error) bool {
	return dav.WasResent(err) &&
		(dav.IsStatus(err, http.StatusMethodNotAllowed) || dav.IsStatus(err, http.StatusPreconditionFailed))
}
